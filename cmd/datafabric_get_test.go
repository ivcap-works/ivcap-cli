// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
)

const getTestAspectID = "urn:ivcap:aspect:abc"

func getTestContent() map[string]any {
	return map[string]any{
		"accession": "GCF_000001735.4",
		"organism":  map[string]any{"organism_name": "Arabidopsis thaliana"},
	}
}

func getTestRecord(withContent bool) map[string]any {
	record := map[string]any{
		"id":           getTestAspectID,
		"entity":       "urn:ivcap:artifact:xyz",
		"schema":       "urn:ivcap:schema:ncbi.assembly-data-report.1",
		"asserter":     "urn:ivcap:user:me",
		"valid-from":   "2026-10-17T00:00:00Z",
		"content-type": "application/json",
	}
	if withContent {
		record["content"] = getTestContent()
	}
	return record
}

// newAspectRecordAdapter serves one aspect record and sets the package state that printing reads.
func newAspectRecordAdapter(t *testing.T, record map[string]any) *adpt.Adapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/aspects/") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(record)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	oldLogger, oldNoHistory, oldHistory := logger, noHistory, history
	logger, noHistory, history = log.NewNop(), false, nil
	t.Cleanup(func() { logger, noHistory, history = oldLogger, oldNoHistory, oldHistory })
	ad := adpt.RestAdapter(adpt.WithConnContext(&adpt.ConnectionCtxt{URL: srv.URL, TimeoutSec: 5}))
	return &ad
}

func runAspectGet(t *testing.T, ad *adpt.Adapter, format string, fullRecord bool) (string, error) {
	t.Helper()
	var err error
	out := captureStdout(t, func() {
		err = getAspectWith(context.Background(), ad, getTestAspectID, format, fullRecord)
	})
	return out, err
}

func TestAspectGetPrintsOnlyTheContentByDefault(t *testing.T) {
	ad := newAspectRecordAdapter(t, getTestRecord(true))

	for _, format := range []string{"", "yaml"} {
		out, err := runAspectGet(t, ad, format, false)
		if err != nil {
			t.Fatalf("format %q: %v", format, err)
		}
		for _, want := range []string{"accession: GCF_000001735.4", "organism_name: Arabidopsis thaliana"} {
			if !strings.Contains(out, want) {
				t.Errorf("format %q: the content must be printed as YAML, lacks %q:\n%s", format, want, out)
			}
		}
		for _, unwanted := range []string{getTestAspectID, "urn:ivcap:artifact:xyz", "asserter", "valid-from", "schema"} {
			if strings.Contains(out, unwanted) {
				t.Errorf("format %q: the record's own fields must not be printed, found %q:\n%s", format, unwanted, out)
			}
		}
	}

	out, err := runAspectGet(t, ad, "json", false)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(got, getTestContent()) {
		t.Errorf("--output json must print exactly the content, got %v", got)
	}
}

func TestAspectGetFullRecord(t *testing.T) {
	ad := newAspectRecordAdapter(t, getTestRecord(true))

	out, err := runAspectGet(t, ad, "json", true)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	for key, want := range map[string]string{"id": getTestAspectID, "entity": "urn:ivcap:artifact:xyz", "asserter": "urn:ivcap:user:me"} {
		if rec[key] != want {
			t.Errorf("the full record must carry %s=%q, got %v", key, want, rec[key])
		}
	}
	if _, ok := rec["content"]; !ok {
		t.Errorf("the full record must carry the content: %v", rec)
	}

	out, err = runAspectGet(t, ad, "yaml", true)
	if err != nil || !strings.Contains(out, "entity: urn:ivcap:artifact:xyz") || !strings.Contains(out, "content:") {
		t.Errorf("the full record as YAML: %q (%v)", out, err)
	}

	// without a format the full record is the human readable table
	out, err = runAspectGet(t, ad, "", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Entity", "urn:ivcap:artifact:xyz", "Content", "GCF_000001735.4", getTestAspectID + " (@"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table lacks %q:\n%s", want, out)
		}
	}
}

func TestAspectGetWithoutContentSaysHowToSeeTheRecord(t *testing.T) {
	ad := newAspectRecordAdapter(t, getTestRecord(false))
	if _, err := runAspectGet(t, ad, "json", false); err == nil || !strings.Contains(err.Error(), "--full-record") {
		t.Errorf("a record without content must point at --full-record, got %v", err)
	}
}

func TestAspectGetFlags(t *testing.T) {
	if f := aspectGetCmd.Flags().Lookup("full-record"); f == nil || f.DefValue != "false" {
		t.Errorf("get must have --full-record, off by default: %+v", f)
	}
	if aspectGetCmd.Flags().Lookup("content-only") != nil {
		t.Error("--content-only is the default now, the flag must be gone")
	}
	if datafabricQueryCmd.Flags().Lookup("full-record") == nil {
		t.Error("query --get-if-one prints like get, so it needs --full-record too")
	}
}
