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
	"context"
	"strings"
	"testing"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
)

const (
	dsURN   = "urn:ivcap:dataset:11111111-1111-1111-1111-111111111111"
	projURN = "urn:ivcap:project:22222222-2222-2222-2222-222222222222"
)

func resetDatasetFlags() {
	datasetProject, datasetName, datasetDescription = "", "", ""
	datasetGranteeProject, datasetGranteeService, datasetAccess = "", "", ""
	datasetEveryone = false
}

func TestDatasetSDKShaping(t *testing.T) {
	srv, captured := recordingServer(t, "")
	defer srv.Close()
	setTestContext(t, srv.URL, projURN)
	ctx := context.Background()
	adpt := CreateAdapter(true)
	proj := projURN

	if _, err := sdk.ListProjectDatasetsRaw(ctx, projURN, adpt, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := sdk.CreateDatasetRaw(ctx, projURN, &accountsapi.CreateDatasetPayload{Name: "d"}, adpt, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := sdk.AddDatasetGrantRaw(ctx, dsURN, &accountsapi.DatasetGrantPayload{GranteeKind: "project", GranteeId: &proj, Access: "write"}, adpt, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := sdk.RemoveDatasetGrantRaw(ctx, dsURN, &accountsapi.DatasetGrantPayload{GranteeKind: "public", Access: "read"}, adpt, logger); err != nil {
		t.Fatal(err)
	}

	reqs := captured()
	if r := findReq(reqs, "GET", "/projects/"+projURN+"/datasets"); r == nil {
		t.Errorf("no project dataset list request: %+v", reqs)
	}
	if r := findReq(reqs, "POST", "/projects/"+projURN+"/datasets"); r == nil || r.body["name"] != "d" {
		t.Errorf("bad create request: %+v", r)
	}
	if r := findReq(reqs, "POST", "/datasets/"+dsURN+"/grants"); r == nil ||
		r.body["grantee_kind"] != "project" || r.body["grantee_id"] != projURN || r.body["access"] != "write" {
		t.Errorf("bad grant request: %+v", r)
	}
	r := findReq(reqs, "DELETE", "/datasets/"+dsURN+"/grants")
	if r == nil || !strings.Contains(r.rawQuery, "grantee_kind=public") || !strings.Contains(r.rawQuery, "access=read") ||
		strings.Contains(r.rawQuery, "grantee_id") {
		t.Errorf("bad revoke request: %+v", r)
	}
}

func TestDatasetGrantValidation(t *testing.T) {
	cases := []struct {
		kind, id, access string
		ok               bool
	}{
		{"project", projURN, "read", true},
		{"service", "urn:ivcap:service:s", "write", true},
		{"public", "", "read", true},
		{"public", "", "write", false},
		{"public", projURN, "read", false},
		{"project", "", "read", false},
		{"project", projURN, "admin", false},
		{"user", "u", "read", false},
	}
	for _, c := range cases {
		err := sdk.ValidateDatasetGrant(c.kind, c.id, c.access)
		if (err == nil) != c.ok {
			t.Errorf("ValidateDatasetGrant(%q,%q,%q) = %v, want ok=%v", c.kind, c.id, c.access, err, c.ok)
		}
	}
}

func TestDatasetURNOnly(t *testing.T) {
	setTestContext(t, "http://unused", projURN)
	for _, bad := range []string{"", "11111111-1111-1111-1111-111111111111", projURN, "urn:ivcap:dataset:"} {
		if err := sdk.ValidateDatasetURN(bad); err == nil {
			t.Errorf("ValidateDatasetURN(%q) accepted", bad)
		}
	}
	if err := sdk.ValidateDatasetURN(dsURN); err != nil {
		t.Errorf("valid URN rejected: %v", err)
	}
	if err := readDatasetCmd.RunE(readDatasetCmd, []string{projURN}); err == nil {
		t.Error("get accepted a project URN")
	}
}

func TestDatasetGrantRequestFlags(t *testing.T) {
	setTestContext(t, "http://unused", projURN)
	defer resetDatasetFlags()

	cases := []struct {
		name  string
		setup func()
		want  string // substring of the error, "" for success
	}{
		{"everyone read", func() { datasetEveryone, datasetAccess = true, "read" }, ""},
		{"everyone write rejected", func() { datasetEveryone, datasetAccess = true, "write" }, "read-only"},
		{"no grantee", func() { datasetAccess = "read" }, "exactly one"},
		{"two grantees", func() { datasetEveryone, datasetGranteeProject, datasetAccess = true, projURN, "read" }, "exactly one"},
		{"missing access", func() { datasetGranteeProject = projURN }, "access must be"},
		{"project write", func() { datasetGranteeProject, datasetAccess = projURN, "write" }, ""},
	}
	for _, c := range cases {
		resetDatasetFlags()
		c.setup()
		_, _, err := datasetGrantRequest(dsURN)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: error %v, want %q", c.name, err, c.want)
		}
	}
}

func TestDatasetDeleteUnsupportedMakesNoRequest(t *testing.T) {
	srv, captured := recordingServer(t, "")
	defer srv.Close()
	setTestContext(t, srv.URL, projURN)

	err := deleteDatasetCmd.RunE(deleteDatasetCmd, []string{dsURN})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("delete: %v, want unsupported error", err)
	}
	if reqs := captured(); len(reqs) != 0 {
		t.Errorf("delete made requests: %+v", reqs)
	}
}

func TestDatasetListDefaultsToCurrentProject(t *testing.T) {
	srv, captured := recordingServer(t, `{"datasets":[]}`)
	defer srv.Close()
	setTestContext(t, srv.URL, projURN)
	outputFormat = "json"
	defer func() { outputFormat = "" }()

	if err := listDatasetCmd.RunE(listDatasetCmd, nil); err != nil {
		t.Fatal(err)
	}
	if findReq(captured(), "GET", "/projects/"+projURN+"/datasets") == nil {
		t.Errorf("did not list the current project's datasets: %+v", captured())
	}

	setTestContext(t, srv.URL, "")
	if err := listDatasetCmd.RunE(listDatasetCmd, nil); err == nil {
		t.Error("list with no project succeeded")
	}
}
