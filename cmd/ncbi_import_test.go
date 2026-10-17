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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	ncbigenome "github.com/ivcap-works/ivcap-ncbi-genome-download"
	log "go.uber.org/zap"
)

const (
	importTestArtifact = "urn:ivcap:artifact:11111111-1111-1111-1111-111111111111"
	importTestReport   = `{"accession":"GCF_000001735.4","organism":{"organism_name":"Arabidopsis thaliana"},"assembly_info":{"release_date":"2018-03-15"}}`
	importTestAspect1  = "urn:ivcap:aspect:00000001-1111-1111-1111-111111111111"
)

// importFixture is a stand-in for NCBI and for IVCAP at once, with switches to make single steps
// of an import fail.
type importFixture struct {
	t       *testing.T
	srv     *httptest.Server
	adapter adpt.Adapter
	tmp     string // the temporary directory of the test; an import must leave nothing behind in it

	mu           sync.Mutex
	requests     []string
	reportCalls  int
	aspectPosts  int
	aspects      []map[string]any
	reportsFor   func(call int) string // body of the n-th (1-based) dataset_report reply
	failUpload   bool
	failAspectAt int // 1-based number of the aspect request that fails; 0 = none
}

func newImportFixture(t *testing.T) *importFixture {
	t.Helper()
	f := &importFixture{t: t, tmp: t.TempDir()}
	f.reportsFor = func(int) string { return `{"reports":[` + importTestReport + `]}` }
	for _, env := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(env, f.tmp)
	}
	zipBytes := ncbiTestZip(t, map[string]string{"ncbi_dataset/data/GCF_000001735.4/genomic.fna": ">chr1\nACGT\n"})
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		switch {
		case r.URL.Path == "/genome/dataset_report":
			f.reportCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, f.reportsFor(f.reportCalls))
		case r.URL.Path == "/genome/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		case r.Method == http.MethodPost && r.URL.Path == "/1/artifacts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":%q,"data-href":%q}`, importTestArtifact, f.srv.URL+"/upload/package")
		case r.Method == http.MethodPatch && r.URL.Path == "/upload/package":
			data, _ := io.ReadAll(r.Body)
			if f.failUpload {
				http.Error(w, "upload refused", http.StatusBadRequest)
				return
			}
			w.Header().Set("Upload-Offset", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/1/aspects":
			f.aspectPosts++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f.aspectPosts == f.failAspectAt {
				http.Error(w, "aspect refused", http.StatusBadRequest)
				return
			}
			f.aspects = append(f.aspects, body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":"urn:ivcap:aspect:%08d-1111-1111-1111-111111111111"}`, f.aspectPosts)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	f.adapter = adpt.RestAdapter(adpt.WithConnContext(&adpt.ConnectionCtxt{URL: f.srv.URL, TimeoutSec: 5}))
	oldLogger := logger
	logger = log.NewNop()
	t.Cleanup(func() { logger = oldLogger })
	return f
}

func baseImportOpts() ncbiImportOpts {
	return ncbiImportOpts{
		Accession:      "GCF_000001735.4",
		ChunkSize:      -1,
		ReportSchema:   ncbiAssemblyReportSchema,
		ManifestSchema: artifactArchiveManifestSchema,
		Format:         "json",
		Silent:         true,
	}
}

func (f *importFixture) run(o ncbiImportOpts) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	err = runNcbiImport(context.Background(), ncbiTestClient(f.srv), o, &f.adapter, &out, &errOut)
	return out.String(), errOut.String(), err
}

func (f *importFixture) count(request string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if r == request {
			n++
		}
	}
	return n
}

func (f *importFixture) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *importFixture) expectNoLeftovers() {
	f.t.Helper()
	if left, _ := filepath.Glob(filepath.Join(f.tmp, "ivcap-ncbi-import-*")); len(left) != 0 {
		f.t.Errorf("the import left temporary files behind: %v", left)
	}
}

func mustContain(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want an error containing %q, got none", wants)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestRunNcbiImportUploadFailure(t *testing.T) {
	f := newImportFixture(t)
	f.failUpload = true
	_, _, err := f.run(baseImportOpts())
	mustContain(t, err, "cannot upload artifact", importTestArtifact)
	if n := f.count("POST /1/aspects"); n != 0 {
		t.Errorf("no aspect may be added to a partly uploaded artifact, got %d", n)
	}
	f.expectNoLeftovers()
}

func TestRunNcbiImportReportAspectFailure(t *testing.T) {
	f := newImportFixture(t)
	f.failAspectAt = 1
	_, _, err := f.run(baseImportOpts())
	mustContain(t, err, importTestArtifact, "report aspect could not be added")
	if n := f.count("POST /1/aspects"); n != 1 {
		t.Errorf("the manifest aspect must not be tried after the report aspect failed, got %d aspect requests", n)
	}
	f.expectNoLeftovers()
}

func TestRunNcbiImportManifestAspectFailure(t *testing.T) {
	f := newImportFixture(t)
	f.failAspectAt = 2
	_, _, err := f.run(baseImportOpts())
	mustContain(t, err, importTestArtifact, importTestAspect1, "manifest aspect could not be added")
	f.expectNoLeftovers()
}

func TestRunNcbiImportRejectsBioProjectUpFront(t *testing.T) {
	for _, accession := range []string{"PRJNA10719", "prjeb1234", "PRJDB5"} {
		f := newImportFixture(t)
		o := baseImportOpts()
		o.Accession = accession
		_, _, err := f.run(o)
		if !errors.Is(err, ncbigenome.ErrNotSingleGenome) {
			t.Errorf("%s: want ErrNotSingleGenome, got %v", accession, err)
		}
		mustContain(t, err, "BioProject", "ivcap genome reports")
		if n := f.total(); n != 0 {
			t.Errorf("%s: nothing may be requested before the accession is checked, got %d requests", accession, n)
		}
	}
}

func TestRunNcbiImportRejectsSeveralAssemblies(t *testing.T) {
	f := newImportFixture(t)
	f.reportsFor = func(int) string {
		return `{"reports":[` + importTestReport + `,{"accession":"GCF_000001736.1"}]}`
	}
	_, _, err := f.run(baseImportOpts())
	if !errors.Is(err, ncbigenome.ErrNotSingleGenome) {
		t.Errorf("want ErrNotSingleGenome, got %v", err)
	}
	mustContain(t, err, "GCF_000001735.4", "GCF_000001736.1", "2 assemblies", "ivcap genome reports")
	if f.count("POST /genome/download") != 0 || f.count("POST /1/artifacts") != 0 {
		t.Errorf("nothing may be downloaded or created, got %v", f.requests)
	}
}

func TestRunNcbiImportNoAssemblies(t *testing.T) {
	f := newImportFixture(t)
	f.reportsFor = func(int) string { return `{"reports":[]}` }
	_, _, err := f.run(baseImportOpts())
	if !errors.Is(err, ncbigenome.ErrNoAssemblies) {
		t.Errorf("want ErrNoAssemblies, got %v", err)
	}
	if f.count("POST /genome/download") != 0 || f.count("POST /1/artifacts") != 0 {
		t.Errorf("nothing may be downloaded or created, got %v", f.requests)
	}
}

func TestRunNcbiImportReportAndPackageMustAgree(t *testing.T) {
	f := newImportFixture(t)
	// the first lookup (the report) and every later one (the download) disagree
	f.reportsFor = func(call int) string {
		if call == 1 {
			return `{"reports":[` + importTestReport + `]}`
		}
		return `{"reports":[{"accession":"GCF_000001735.5","assembly_info":{"release_date":"2018-03-15"}}]}`
	}
	_, _, err := f.run(baseImportOpts())
	mustContain(t, err, "GCF_000001735.4", "GCF_000001735.5", "nothing was imported")
	if n := f.count("POST /1/artifacts"); n != 0 {
		t.Errorf("no artifact may be created when the report and the package disagree, got %d", n)
	}
	f.expectNoLeftovers()
}

func TestRunNcbiImportRejectsBadLicenseUpFront(t *testing.T) {
	f := newImportFixture(t)
	o := baseImportOpts()
	o.License = "not a license!"
	_, _, err := f.run(o)
	mustContain(t, err, "invalid --license")
	if n := f.total(); n != 0 {
		t.Errorf("nothing may be requested with an invalid license, got %d requests", n)
	}
}

func TestRunNcbiImportLicenseOverride(t *testing.T) {
	f := newImportFixture(t)
	o := baseImportOpts()
	o.License = "CC0-1.0"
	if _, _, err := f.run(o); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.aspects) != 2 {
		t.Fatalf("want 2 aspects, got %d", len(f.aspects))
	}
	graph, _ := f.aspects[1]["ro_crate"].(map[string]any)["@graph"].([]any)
	root, _ := graph[1].(map[string]any)
	last, _ := graph[len(graph)-1].(map[string]any)
	if lic, _ := root["license"].(map[string]any); lic["@id"] != "https://spdx.org/licenses/CC0-1.0" {
		t.Errorf("the root must point at the given license, got %v", root["license"])
	}
	if last["@id"] != "https://spdx.org/licenses/CC0-1.0" || last["name"] != "CC0-1.0" {
		t.Errorf("unexpected license entity: %v", last)
	}
}

func TestRunNcbiImportHumanOutput(t *testing.T) {
	f := newImportFixture(t)
	o := baseImportOpts()
	o.Format, o.Silent = "", false
	stdout, stderr, err := f.run(o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "Uploading GCF_000001735.4.zip") {
		t.Errorf("the upload must announce itself on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Downloaded and imported GCF_000001735.4 as "+importTestArtifact) {
		t.Errorf("unexpected summary: %q", stdout)
	}
	f.expectNoLeftovers()
}

func TestNcbiImportFlagDefaultsMatchTheSchemas(t *testing.T) {
	for flag, want := range map[string]string{
		"report-schema":   ncbiAssemblyReportSchema,
		"manifest-schema": artifactArchiveManifestSchema,
	} {
		if f := ncbiImportCmd.Flags().Lookup(flag); f == nil || f.DefValue != want {
			t.Errorf("--%s must default to %q, got %+v", flag, want, f)
		}
	}
}
