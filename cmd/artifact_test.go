// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"

	log "go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
)

func TestListArtifact(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	req := &sdk.ListArtifactRequest{Offset: 0, Limit: 5}
	_, err := sdk.ListArtifacts(context.Background(), req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to list artifacts: %v", err)
	}
}

var testArtifactID string

const testArtifactFileName = "./test_data/img"

func testAddArtifact(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	reader, contentType, size := getReader(testArtifactFileName, contentType)
	req := &sdk.CreateArtifactRequest{
		Name:       artifactName,
		Size:       size,
		Collection: artifactCollection,
		Policy:     policy,
	}
	resp, err := sdk.CreateArtifact(context.Background(), req, contentType, size, nil, adapter, logger)
	if err != nil {
		t.Fatalf("while creating record for '%s'- %v", testArtifactFileName, err)
	}
	if resp.ID == nil || *resp.ID == "" {
		t.Fatalf("created artifact response does not have ID")
	}
	testArtifactID = *resp.ID

	path, err := (*adapter).GetPath(*resp.Data.Self)
	if err != nil {
		t.Fatalf("while parsing API reply - %v", err)
	}
	if err = upload(context.Background(), reader, testArtifactID, path, size, 0, adapter); err != nil {
		t.Fatalf("while upload - %v", err)
	}
}

func testGetArtifact(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	if testArtifactID == "" {
		t.Skip("artifactID not set ...")
	}

	req := &sdk.ReadArtifactRequest{Id: testArtifactID}
	artifact, err := sdk.ReadArtifact(context.Background(), req, adapter, logger)
	if err != nil {
		t.Fatalf("failed to get artifact: %v", err)
	}
	if artifact.Data == nil || artifact.Data.Self == nil {
		t.Fatalf("No data available")
	}
	if artifact.ID == nil || *artifact.ID != testArtifactID {
		t.Fatalf("artifact id does not match, expected: %s, got: %s", testArtifactID, *artifact.ID)
	}
	selector := sdk.MetadataSelector{Entity: recordID}
	if _, _, err := sdk.ListMetadata(context.Background(), selector, adapter, logger); err != nil {
		t.Fatalf("error while list artifact metadata: %v", err)
	}

	u, err := url.ParseRequestURI(*artifact.Data.Self)
	if err != nil {
		t.Fatalf("error parsing url: %s, %v", *artifact.Data.Self, err)
	}

	downloadHandler := func(resp *http.Response, path string, logger *log.Logger) error {
		if resp.StatusCode >= 300 {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
			return fmt.Errorf("failed to download, statusCode: %d, error: %v", resp.StatusCode, string(data))
		}

		expected, err := os.ReadFile(testArtifactFileName)
		if err != nil {
			return fmt.Errorf("failed to read artifact data file: %w", err)
		}

		result, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if !bytes.Equal(result, expected) {
			return fmt.Errorf("unexpected download result: %s, expected: %s",
				base64.StdEncoding.EncodeToString(result), base64.StdEncoding.EncodeToString(expected))
		}

		return nil
	}

	if err := (*adapter).GetWithHandler(context.Background(), u.Path, nil, downloadHandler, logger); err != nil {
		t.Fatalf("failed to download artifact: %v", err)
	}
}
