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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/aspect"
	log "go.uber.org/zap"
)

const (
	listArtifact1 = "urn:ivcap:artifact:11111111-1111-1111-1111-111111111111"
	listArtifact2 = "urn:ivcap:artifact:22222222-2222-2222-2222-222222222222"
	listAspect1   = "urn:ivcap:aspect:aaaaaaaa-1111-1111-1111-111111111111"
	listAspect2   = "urn:ivcap:aspect:bbbbbbbb-2222-2222-2222-222222222222"
)

type listTestItem struct {
	id, entity, schema string
	content            any
}

// listTestItems: two imports, one aspect whose schema only shares the prefix (the API returns
// those too), and one report without an accession.
func listTestItems() []listTestItem {
	return []listTestItem{
		{listAspect1, listArtifact1, ncbiAssemblyReportSchema, map[string]any{
			"accession":     "GCF_000001735.4",
			"organism":      map[string]any{"organism_name": "Arabidopsis thaliana"},
			"assembly_info": map[string]any{"assembly_name": "TAIR10.1"},
		}},
		{listAspect2, listArtifact2, ncbiAssemblyReportSchema, map[string]any{"accession": "GCA_000001735.2"}},
		{"urn:ivcap:aspect:cccccccc-3333-3333-3333-333333333333", "urn:ivcap:artifact:33333333-3333-3333-3333-333333333333",
			ncbiAssemblyReportSchema + "0", map[string]any{"accession": "GCF_999999999.1"}},
		{"urn:ivcap:aspect:dddddddd-4444-4444-4444-444444444444", "urn:ivcap:artifact:44444444-4444-4444-4444-444444444444",
			ncbiAssemblyReportSchema, map[string]any{"other": "x"}},
	}
}

// listReplyJSON builds an aspect list reply with the generated API type itself, so the test does
// not depend on guessed JSON field names.
func listReplyJSON(items []listTestItem, nextPage string) ([]byte, error) {
	var list api.ListResponseBody
	slice := reflect.ValueOf(&list).Elem().FieldByName("Items")
	elem := slice.Type().Elem()
	isPtr := elem.Kind() == reflect.Ptr
	for _, it := range items {
		var item reflect.Value
		if isPtr {
			item = reflect.New(elem.Elem())
		} else {
			item = reflect.New(elem)
		}
		for field, value := range map[string]string{"ID": it.id, "Entity": it.entity, "Schema": it.schema} {
			v := value
			item.Elem().FieldByName(field).Set(reflect.ValueOf(&v))
		}
		if it.content != nil {
			item.Elem().FieldByName("Content").Set(reflect.ValueOf(it.content))
		}
		if isPtr {
			slice.Set(reflect.Append(slice, item))
		} else {
			slice.Set(reflect.Append(slice, item.Elem()))
		}
	}
	if nextPage != "" {
		rel, href := "next", "https://example.org/1/aspects?page="+url.QueryEscape(nextPage)
		list.Links = []*api.LinkTResponseBody{{Rel: &rel, Href: &href}}
	}
	return json.Marshal(list)
}

type listFixture struct {
	adapter adpt.Adapter
	mu      sync.Mutex
	queries []url.Values
}

func newListFixture(t *testing.T, items []listTestItem, nextPage string) *listFixture {
	t.Helper()
	body, err := listReplyJSON(items, nextPage)
	if err != nil {
		t.Fatal(err)
	}
	f := &listFixture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == http.MethodGet && r.URL.Path == "/1/aspects" {
			f.queries = append(f.queries, r.URL.Query())
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	f.adapter = adpt.RestAdapter(adpt.WithConnContext(&adpt.ConnectionCtxt{URL: srv.URL, TimeoutSec: 5}))
	oldLogger, oldNoHistory, oldHistory := logger, noHistory, history
	logger, noHistory, history = log.NewNop(), false, nil
	t.Cleanup(func() { logger, noHistory, history = oldLogger, oldNoHistory, oldHistory })
	return f
}

func (f *listFixture) run(format string) (string, error) {
	var out bytes.Buffer
	err := runGenomeList(context.Background(), &f.adapter, ncbiAssemblyReportSchema, sdk.ListRequest{Limit: 7}, format, &out)
	return out.String(), err
}

func TestRunGenomeListTable(t *testing.T) {
	f := newListFixture(t, listTestItems(), "")
	out, err := f.run("")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ACCESSION", "REPORT ASPECT", "ARTIFACT", "GCF_000001735.4", "GCA_000001735.2", genomeUnknownAccession} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "GCF_999999999.1") {
		t.Errorf("an aspect with another schema must not be listed:\n%s", out)
	}
	// by default the URNs are replaced by short @n references, which the history resolves
	if strings.Contains(out, "urn:ivcap:") {
		t.Errorf("without --no-history the table must show @n references, not URNs:\n%s", out)
	}
	if n := len(regexp.MustCompile(`@\d+`).FindAllString(out, -1)); n != 6 {
		t.Errorf("want 6 @n references (an aspect and an artifact for each of 3 rows), got %d:\n%s", n, out)
	}
	referenced := map[string]bool{}
	for _, urn := range history {
		referenced[urn] = true
	}
	for _, urn := range []string{listAspect1, listArtifact1, listAspect2, listArtifact2} {
		if !referenced[urn] {
			t.Errorf("%s must be reachable through an @n reference; history = %v", urn, history)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queries) != 1 {
		t.Fatalf("want one data fabric query, got %d", len(f.queries))
	}
	q := f.queries[0]
	if q.Get("schema") != ncbiAssemblyReportSchema || q.Get("include-content") != "true" || q.Get("entity") != "" {
		t.Errorf("the query must ask for the report schema with content and no entity: %v", q)
	}
}

func TestRunGenomeListJSON(t *testing.T) {
	f := newListFixture(t, listTestItems(), "cursor-123")
	out, err := f.run("json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "(@") {
		t.Errorf("machine-readable output must carry plain URNs, got %s", out)
	}
	var got genomeListOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	want := []genomeListRow{
		{Accession: "GCF_000001735.4", Organism: "Arabidopsis thaliana", AssemblyName: "TAIR10.1", ReportAspect: listAspect1, Artifact: listArtifact1},
		{Accession: "GCA_000001735.2", ReportAspect: listAspect2, Artifact: listArtifact2},
		{Accession: genomeUnknownAccession, ReportAspect: "urn:ivcap:aspect:dddddddd-4444-4444-4444-444444444444",
			Artifact: "urn:ivcap:artifact:44444444-4444-4444-4444-444444444444"},
	}
	if !reflect.DeepEqual(got.Genomes, want) {
		t.Errorf("rows = %+v, want %+v", got.Genomes, want)
	}
	if got.NextPage != "cursor-123" {
		t.Errorf("next_page = %q, want the page cursor of the next link", got.NextPage)
	}
	var generic map[string]any
	_ = json.Unmarshal([]byte(out), &generic)
	row, _ := generic["genomes"].([]any)[0].(map[string]any)
	for _, key := range []string{"accession", "organism", "assembly_name", "report_aspect", "artifact"} {
		if _, ok := row[key]; !ok {
			t.Errorf("row lacks the key %q: %v", key, row)
		}
	}
	// a report without organism or assembly name leaves the keys out
	second, _ := generic["genomes"].([]any)[1].(map[string]any)
	if _, ok := second["organism"]; ok {
		t.Errorf("a row without an organism must not carry the key: %v", second)
	}
}

func TestRunGenomeListYAML(t *testing.T) {
	f := newListFixture(t, listTestItems(), "cursor-123")
	out, err := f.run("yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"genomes:", "accession: GCF_000001735.4", "report_aspect: " + listAspect1, "artifact: " + listArtifact1, "next_page: cursor-123"} {
		if !strings.Contains(out, want) {
			t.Errorf("YAML lacks %q:\n%s", want, out)
		}
	}
}

func TestRunGenomeListEmpty(t *testing.T) {
	f := newListFixture(t, nil, "")
	out, err := f.run("")
	if err != nil || !strings.Contains(out, "Genomes") || !strings.Contains(out, "ACCESSION") || strings.Contains(out, "(@") {
		t.Errorf("an empty list must print the usual table without rows, got %q (%v)", out, err)
	}
	out, err = f.run("json")
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(out), &generic); err != nil {
		t.Fatal(err)
	}
	if rows, ok := generic["genomes"].([]any); !ok || len(rows) != 0 {
		t.Errorf("an empty list must print genomes as [] and not null, got %q", out)
	}
}

func TestRunGenomeListNextPageHint(t *testing.T) {
	f := newListFixture(t, listTestItems(), "cursor-123")
	out, err := f.run("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Genomes") {
		t.Errorf("the records belong under a 'Genomes' label like 'Jobs' or 'Records' in the other lists:\n%s", out)
	}
	if !regexp.MustCompile(`\.\.\. @\d+`).MatchString(out) {
		t.Errorf("the table must end with the usual next-page row '... @n':\n%s", out)
	}
}

func TestRunGenomeListRejectsEmptySchema(t *testing.T) {
	var out bytes.Buffer
	if err := runGenomeList(context.Background(), nil, "", sdk.ListRequest{}, "", &out); err == nil {
		t.Error("an empty report schema must be rejected before anything is requested")
	}
}

func TestGenomeListFlags(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"genome", "list"})
	if err != nil || c.Name() != "list" {
		t.Fatalf("genome list not found: %v", err)
	}
	if f := c.Flags().Lookup("report-schema"); f == nil || f.DefValue != ncbiAssemblyReportSchema {
		t.Errorf("--report-schema must default to the schema that import writes, got %+v", f)
	}
	for _, flag := range []string{"limit", "page"} {
		if c.Flags().Lookup(flag) == nil {
			t.Errorf("genome list lacks the common list flag --%s", flag)
		}
	}
	if f := c.InheritedFlags().Lookup("output"); f == nil || f.Shorthand != "o" {
		t.Error("genome list must inherit the global -o/--output")
	}
	if c.Flags().Lookup("ncbi-api-key") != nil || c.InheritedFlags().Lookup("ncbi-api-key") != nil {
		t.Error("genome list never talks to NCBI and must not offer --ncbi-api-key")
	}
}

func TestRunGenomeListNoHistoryShowsFullURNs(t *testing.T) {
	f := newListFixture(t, listTestItems(), "")
	noHistory = true // --no-history; the fixture restores the old value
	out, err := f.run("")
	if err != nil {
		t.Fatal(err)
	}
	for _, urn := range []string{listAspect1, listArtifact1, listAspect2, listArtifact2} {
		if !strings.Contains(out, urn) {
			t.Errorf("with --no-history the table must show %s:\n%s", urn, out)
		}
	}
	if regexp.MustCompile(`@\d+`).MatchString(out) {
		t.Errorf("with --no-history the table must not contain @n references:\n%s", out)
	}
}
