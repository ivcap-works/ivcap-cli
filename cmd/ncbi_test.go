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
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ncbigenome "github.com/ivcap-works/ivcap-ncbi-genome-download"
)

func ncbiTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ncbiTestServer mimics the two NCBI Datasets endpoints the library uses.
func ncbiTestServer(t *testing.T, reportsJSON string, zipBytes []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		switch r.URL.Path {
		case "/genome/dataset_report":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, reportsJSON)
		case "/genome/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func ncbiTestClient(srv *httptest.Server) *ncbigenome.Client {
	return ncbigenome.New(ncbigenome.Options{BaseURL: srv.URL, MaxRetries: -1})
}

func TestNcbiDownloadRequest(t *testing.T) {
	req, err := ncbiDownloadRequest(ncbiDownloadOpts{
		Accession:       "GCF_000001735.4",
		Includes:        []string{"genome", "gff3"},
		Chromosomes:     []string{"1"},
		AssemblyVersion: "current",
		FastZip:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Accessions) != 1 || req.Accessions[0] != "GCF_000001735.4" {
		t.Errorf("accessions = %v", req.Accessions)
	}
	if len(req.Include) != 2 || req.Include[0] != ncbigenome.IncludeGenome || req.Include[1] != ncbigenome.IncludeGFF3 {
		t.Errorf("include = %v", req.Include)
	}
	if req.AssemblyVersion != ncbigenome.VersionCurrent || !req.SkipZipContentCheck {
		t.Errorf("version/fast zip not mapped: %+v", req)
	}
	if len(req.Chromosomes) != 1 || req.Chromosomes[0] != "1" {
		t.Errorf("chromosomes = %v", req.Chromosomes)
	}

	if _, err := ncbiDownloadRequest(ncbiDownloadOpts{Accession: "GCF_1.1", Includes: []string{"bogus"}}); err == nil {
		t.Error("an unknown --include value was accepted")
	}
	if _, err := ncbiDownloadRequest(ncbiDownloadOpts{Accession: "GCF_1.1", AssemblyVersion: "sometimes"}); err == nil {
		t.Error("an unknown --assembly-version value was accepted")
	}
}

func TestNcbiReportsRequest(t *testing.T) {
	req, err := ncbiReportsRequest(ncbiReportsOpts{
		Accessions:          []string{"PRJNA10719", "GCF_000001735.4"},
		AssemblyVersion:     "all",
		Reference:           true,
		Annotated:           true,
		Source:              "refseq",
		Levels:              []string{"complete", "chromosome"},
		MAG:                 "exclude",
		ExcludeAtypical:     true,
		ExcludeMultiIsolate: true,
		FromType:            true,
	})
	if err != nil {
		t.Fatal(err)
	}
	f := req.Filters
	if len(req.Accessions) != 2 || req.AssemblyVersion != ncbigenome.VersionAll {
		t.Errorf("accessions/version not mapped: %+v", req)
	}
	if !f.ReferenceOnly || !f.AnnotatedOnly || !f.ExcludeAtypical || !f.ExcludeMultiIsolate || !f.TypeMaterialOnly {
		t.Errorf("boolean filters not mapped: %+v", f)
	}
	if f.AssemblySource != ncbigenome.SourceRefSeq || f.MAG != ncbigenome.MAGExclude {
		t.Errorf("source/MAG not mapped: %+v", f)
	}
	if len(f.AssemblyLevels) != 2 || f.AssemblyLevels[0] != ncbigenome.LevelComplete {
		t.Errorf("levels = %v", f.AssemblyLevels)
	}

	for name, o := range map[string]ncbiReportsOpts{
		"version": {AssemblyVersion: "x", Source: "all", MAG: "all"},
		"source":  {Source: "ensembl", MAG: "all"},
		"level":   {Source: "all", MAG: "all", Levels: []string{"huge"}},
		"mag":     {Source: "all", MAG: "maybe"},
	} {
		if _, err := ncbiReportsRequest(o); err == nil {
			t.Errorf("an invalid %s value was accepted", name)
		}
	}
}

func TestNcbiDestination(t *testing.T) {
	got, err := ncbiDestination("gcf_000001735.4", "")
	if err != nil || got != "GCF_000001735.4.zip" {
		t.Errorf("default destination = %q (%v)", got, err)
	}
	// The accession must never steer the default path out of the working directory.
	for _, bad := range []string{"../x", "a/b", "GCF 1", ""} {
		if _, err := ncbiDestination(bad, ""); err == nil {
			t.Errorf("accession %q produced a default path", bad)
		}
	}
	if got, _ := ncbiDestination("GCF_1.1", "./out/../x.zip"); got != "x.zip" {
		t.Errorf("explicit path was not cleaned: %q", got)
	}
	if got, _ := ncbiDestination("GCF_1.1", "-"); got != "-" {
		t.Errorf("stdout marker changed: %q", got)
	}
}

func TestNcbiCheckOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.zip")
	if err := ncbiCheckOverwrite(p, false); err != nil {
		t.Errorf("a missing file must be fine: %v", err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ncbiCheckOverwrite(p, false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("an existing file must be refused with a hint, got %v", err)
	}
	if err := ncbiCheckOverwrite(p, true); err != nil {
		t.Errorf("--force must allow overwriting: %v", err)
	}
	if err := ncbiCheckOverwrite("-", false); err != nil {
		t.Errorf("stdout must always be allowed: %v", err)
	}
}

func TestNcbiProgressOff(t *testing.T) {
	var buf bytes.Buffer
	for _, c := range []struct {
		quiet  bool
		format string
	}{{true, ""}, {false, "json"}, {false, "yaml"}} {
		onProgress, done := ncbiProgress(c.quiet, c.format, &buf)
		if onProgress != nil {
			t.Errorf("quiet=%v format=%q must not show progress", c.quiet, c.format)
		}
		done()
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected output %q", buf.String())
	}
}

// YAML marshalling ignores json tags, so printing a library type directly would
// rename its keys. printNcbiStructured converts through JSON to keep them.
func TestNcbiStructuredOutputKeepsJSONNames(t *testing.T) {
	report := ncbigenome.AssemblyDataReport{
		Accession:    "GCF_000001735.4",
		AssemblyInfo: &ncbigenome.AssemblyInfo{AssemblyName: "TAIR10.1"},
	}

	var js bytes.Buffer
	if err := printNcbiStructured(&js, []ncbigenome.AssemblyDataReport{report}, "json"); err != nil {
		t.Fatal(err)
	}
	var reports []map[string]any
	if err := json.Unmarshal(js.Bytes(), &reports); err != nil || len(reports) != 1 {
		t.Fatalf("json output is not a one-element array: %v\n%s", err, js.String())
	}
	info, _ := reports[0]["assembly_info"].(map[string]any)
	if info["assembly_name"] != "TAIR10.1" {
		t.Errorf("json keys changed: %v", reports[0])
	}
	if reports[0]["$schema"] != ncbigenome.AssemblyDataReportSchemaURL {
		t.Errorf("the $schema reference is missing: %v", reports[0]["$schema"])
	}

	var ym bytes.Buffer
	if err := printNcbiStructured(&ym, report, "yaml"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"accession: GCF_000001735.4", "assembly_info:", "assembly_name: TAIR10.1"} {
		if !strings.Contains(ym.String(), want) {
			t.Errorf("yaml output lacks %q:\n%s", want, ym.String())
		}
	}
}

func TestRunNcbiDownloadToFile(t *testing.T) {
	contents := map[string]string{
		"ncbi_dataset/data/assembly_data_report.jsonl":  "{}\n",
		"ncbi_dataset/data/GCF_000001735.4/genomic.gff": "##gff-version 3\n",
	}
	zipBytes := ncbiTestZip(t, contents)
	srv := ncbiTestServer(t, `{"reports":[{"accession":"GCF_000001735.4"}]}`, zipBytes)
	client := ncbiTestClient(srv)
	ctxt := context.Background()

	dest := filepath.Join(t.TempDir(), "out.zip")
	o := ncbiDownloadOpts{Accession: "GCF_000001735.4", File: dest, Includes: []string{"genome", "gff3"}, Silent: true}

	// Human output: the file is the package, and the manifest table lists its files.
	var out, errOut bytes.Buffer
	if err := runNcbiDownload(ctxt, client, o, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Clean(dest))
	if err != nil || !bytes.Equal(got, zipBytes) {
		t.Fatalf("the downloaded file differs from the package (read error: %v)", err)
	}
	// The table renderer upper-cases header and footer text, so compare ignoring case.
	for _, want := range []string{"Downloaded GCF_000001735.4", "assembly_data_report.jsonl", "genomic.gff", "Total"} {
		if !strings.Contains(strings.ToLower(out.String()), strings.ToLower(want)) {
			t.Errorf("human output lacks %q:\n%s", want, out.String())
		}
	}

	// A second run must not replace the file silently.
	err = runNcbiDownload(ctxt, client, o, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("an existing file must be refused with a hint, got %v", err)
	}
	o.Force = true
	if err := runNcbiDownload(ctxt, client, o, &out, &errOut); err != nil {
		t.Errorf("--force must allow overwriting: %v", err)
	}

	// Structured output carries the result and the manifest.
	out.Reset()
	o.Format = "json"
	if err := runNcbiDownload(ctxt, client, o, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json output is not valid JSON: %v\n%s", err, out.String())
	}
	if res["accession"] != "GCF_000001735.4" || res["file"] != dest {
		t.Errorf("unexpected result: %v", res)
	}
	manifest, _ := res["manifest"].(map[string]any)
	files, _ := manifest["files"].([]any)
	if len(files) != len(contents) {
		t.Errorf("manifest lists %d files, want %d", len(files), len(contents))
	}
}

func TestRunNcbiDownloadToStdout(t *testing.T) {
	zipBytes := ncbiTestZip(t, map[string]string{"a.txt": "x"})
	srv := ncbiTestServer(t, `{"reports":[{"accession":"GCF_000001735.4"}]}`, zipBytes)
	client := ncbiTestClient(srv)

	var out, errOut bytes.Buffer
	o := ncbiDownloadOpts{Accession: "GCF_000001735.4", File: "-", Silent: true}
	if err := runNcbiDownload(context.Background(), client, o, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), zipBytes) {
		t.Error("stdout must hold exactly the zip and nothing else")
	}

	// stdout is taken by the package, so a structured result cannot share it.
	o.Format = "json"
	err := runNcbiDownload(context.Background(), client, o, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "stdout") {
		t.Errorf("'-f -' with --output must be refused, got %v", err)
	}
}

func TestRunNcbiDownloadRejectsMoreThanOneGenome(t *testing.T) {
	srv := ncbiTestServer(t, `{"reports":[]}`, nil)
	client := ncbiTestClient(srv)
	dest := filepath.Join(t.TempDir(), "x.zip")

	var out, errOut bytes.Buffer
	o := ncbiDownloadOpts{Accession: "PRJNA10719", File: dest, Silent: true}
	err := runNcbiDownload(context.Background(), client, o, &out, &errOut)
	if !errors.Is(err, ncbigenome.ErrNotSingleGenome) {
		t.Fatalf("a BioProject must be rejected as not a single genome, got %v", err)
	}
	if !strings.Contains(err.Error(), "ivcap ncbi genome reports") || strings.Contains(err.Error(), "AssemblyReports") {
		t.Errorf("the hint must name the CLI command, not the Go API: %v", err)
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("a rejected request must not create the file")
	}
}

func TestRunNcbiReports(t *testing.T) {
	reports := `{"reports":[` +
		`{"accession":"GCF_000001735.4","source_database":"SOURCE_DATABASE_REFSEQ",` +
		`"organism":{"organism_name":"Arabidopsis thaliana"},` +
		`"assembly_info":{"assembly_name":"TAIR10.1","assembly_level":"Chromosome","assembly_status":"current","release_date":"2018-03-15"}},` +
		`{"accession":"GCA_000001735.2"}]}`
	srv := ncbiTestServer(t, reports, nil)
	client := ncbiTestClient(srv)
	ctxt := context.Background()
	o := ncbiReportsOpts{Accessions: []string{"PRJNA10719"}, Source: "all", MAG: "all"}

	var out bytes.Buffer
	if err := runNcbiReports(ctxt, client, o, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GCF_000001735.4", "Arabidopsis thaliana", "TAIR10.1", "REFSEQ", "2018-03-15", "GCA_000001735.2"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("table lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	o.Format = "json"
	if err := runNcbiReports(ctxt, client, o, &out); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 2 {
		t.Fatalf("want a two-element JSON array, got %v\n%s", err, out.String())
	}
	if got[0]["accession"] != "GCF_000001735.4" || got[0]["$schema"] == nil {
		t.Errorf("unexpected first report: %v", got[0])
	}
}

func TestRunNcbiReportsEmpty(t *testing.T) {
	srv := ncbiTestServer(t, `{"reports":[]}`, nil)
	client := ncbiTestClient(srv)
	o := ncbiReportsOpts{Accessions: []string{"GCF_000001735.4"}, Source: "all", MAG: "all"}

	var out bytes.Buffer
	if err := runNcbiReports(context.Background(), client, o, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No matching assemblies") {
		t.Errorf("human output = %q", out.String())
	}

	out.Reset()
	o.Format = "json"
	if err := runNcbiReports(context.Background(), client, o, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("an empty result must print [] not null, got %q", out.String())
	}
}

// -o/--output is the global output-format flag; the ncbi commands must inherit
// it and must not define their own (as 'nextflow job-result' does).
func TestNcbiCommandWiring(t *testing.T) {
	for _, path := range [][]string{{"ncbi", "genome", "download"}, {"ncbi", "genome", "reports"}} {
		c, _, err := rootCmd.Find(path)
		if err != nil || c == nil || c.Name() != path[len(path)-1] {
			t.Fatalf("command %v not found: %v", path, err)
		}
		if c.LocalFlags().Lookup("output") != nil {
			t.Errorf("%v must not define its own --output", path)
		}
		if f := c.InheritedFlags().Lookup("output"); f == nil || f.Shorthand != "o" {
			t.Errorf("%v must inherit the global -o/--output", path)
		}
	}
	c, _, _ := rootCmd.Find([]string{"ncbi", "genome", "download"})
	if f := c.Flags().Lookup("file"); f == nil || f.Shorthand != "f" {
		t.Error("download must take the destination as -f/--file, like 'artifact download'")
	}
}

func TestNcbiCLIErr(t *testing.T) {
	if ncbiCLIErr(nil) != nil {
		t.Error("nil must stay nil")
	}
	plain := errors.New("unrelated")
	if got := ncbiCLIErr(plain); got != plain {
		t.Errorf("an unrelated error must pass through unchanged, got %v", got)
	}

	lib := errors.Join(ncbigenome.ErrNotSingleGenome, errors.New("use AssemblyReports to see which assemblies it has"))
	got := ncbiCLIErr(lib)
	if !errors.Is(got, ncbigenome.ErrNotSingleGenome) {
		t.Error("the library error must stay reachable through errors.Is")
	}
	if !strings.Contains(got.Error(), "'ivcap ncbi genome reports'") || strings.Contains(got.Error(), "AssemblyReports") {
		t.Errorf("wording was not adapted: %v", got)
	}
}
