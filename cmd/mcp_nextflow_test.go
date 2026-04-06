// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	artifact "github.com/ivcap-works/ivcap-core-api/http/artifact"
	log "go.uber.org/zap"
)

func TestTarGzFromNextflowSources_AssemblesTextAndArtifactInnerPath(t *testing.T) {
	defer func(
		oldFetch func(context.Context, string, *a.Adapter) ([]byte, error),
		oldRead func(context.Context, *sdk.ReadArtifactRequest, *a.Adapter, *log.Logger) (*artifact.ReadResponseBody, error),
	) {
		downloadArtifactBytesFn = oldFetch
		readArtifactFn = oldRead
	}(downloadArtifactBytesFn, readArtifactFn)

	// Provide an in-memory tar.gz artifact with inner file.
	tarball := makeTarGz(map[string][]byte{"inner/main.nf": []byte("process X {}")})
	downloadArtifactBytesFn = func(_ context.Context, _ string, _ *a.Adapter) ([]byte, error) { return tarball, nil }
	readArtifactFn = func(_ context.Context, _ *sdk.ReadArtifactRequest, _ *a.Adapter, _ *log.Logger) (*artifact.ReadResponseBody, error) {
		u := "http://example.com/data"
		mt := "application/gzip"
		return &artifact.ReadResponseBody{DataHref: &u, MimeType: &mt}, nil
	}

	sources := []nextflowSource{
		{Path: "ivcap-tool.yaml", Type: "text", Text: testToolYAML},
		{Path: "main.nf", Type: "artifact", ArtifactID: "urn:ivcap:artifact:1", ArtifactPath: "inner/main.nf"},
	}

	b, _, err := tarGzFromNextflowSources(context.Background(), sources, (*a.Adapter)(nil))
	if err != nil {
		t.Fatalf("tarGzFromNextflowSources: %v", err)
	}

	// Verify tar contains paths we expect.
	gzr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tar read: %v", err)
		}
		seen[h.Name] = true
	}
	if !seen["ivcap-tool.yaml"] {
		t.Fatalf("expected ivcap-tool.yaml")
	}
	if !seen["main.nf"] {
		t.Fatalf("expected main.nf")
	}
	if !seen["MANIFEST.json"] {
		t.Fatalf("expected MANIFEST.json")
	}
}

func TestLoadNextflowToolHeaderFromArchiveBytes_PrefersIvcapYaml(t *testing.T) {
	// Both ivcap.yaml and ivcap-tool.yaml exist; ivcap.yaml should win.
	tgz := makeTarGz(map[string][]byte{
		nextflowSimpleToolFileName: []byte(testSimpleToolYAML),
		nextflowToolFileName:       []byte(testToolYAML),
	})
	tool, found, err := loadNextflowToolHeaderFromArchiveBytes(tgz)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tool == nil {
		t.Fatalf("expected tool")
	}
	if found != nextflowSimpleToolFileName {
		t.Fatalf("expected %s, got %s", nextflowSimpleToolFileName, found)
	}
	if tool.FnSchema == nil {
		t.Fatalf("expected generated fn-schema")
	}
}
