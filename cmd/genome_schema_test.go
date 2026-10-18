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
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	ncbigenome "github.com/ivcap-works/ivcap-ncbi-genome-download"
)

// These tests tie the JSON Schema files in schema/ to the constants the code writes into
// aspects, so the two cannot drift apart unnoticed.

func readSchemaFile(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "schema", name))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("%s is not valid JSON: %v", name, err)
	}
	return doc
}

// schemaAt follows a path of object keys (strings) and array indexes (ints) into a schema.
func schemaAt(t *testing.T, doc any, path ...any) string {
	t.Helper()
	cur := doc
	for _, step := range path {
		switch s := step.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("schema path %v: not an object at %q", path, s)
			}
			if cur, ok = m[s]; !ok {
				t.Fatalf("schema path %v: no key %q", path, s)
			}
		case int:
			l, ok := cur.([]any)
			if !ok || s >= len(l) {
				t.Fatalf("schema path %v: no index %d", path, s)
			}
			cur = l[s]
		}
	}
	str, ok := cur.(string)
	if !ok {
		t.Fatalf("schema path %v: not a string: %v", path, cur)
	}
	return str
}

type schemaCheck struct {
	what string
	path []any
	want string
}

func runSchemaChecks(t *testing.T, doc map[string]any, checks []schemaCheck) {
	t.Helper()
	for _, c := range checks {
		if got := schemaAt(t, doc, c.path...); got != c.want {
			t.Errorf("%s: schema has %q, code has %q", c.what, got, c.want)
		}
	}
}

func TestReportSchemaMatchesCode(t *testing.T) {
	doc := readSchemaFile(t, "ncbi-assembly-data-report.1.schema.json")
	lib := ncbigenome.AssemblyDataReportSchemaURL
	runSchemaChecks(t, doc, []schemaCheck{
		{"$id", []any{"$id"}, ncbiAssemblyReportSchema},
		{"$ref to the library schema", []any{"allOf", 0, "$ref"}, lib},
		{"$schema const", []any{"allOf", 1, "properties", "$schema", "const"}, ncbiAssemblyReportSchema},
		{"json_schema const", []any{"allOf", 1, "properties", "json_schema", "const"}, lib},
	})

	// The library schema must be pinned to a release tag, never a branch, and the tag must be
	// the version of the library module this CLI is built with.
	if !regexp.MustCompile(`/ivcap-ncbi-genome-download/v[0-9]+\.[0-9]+\.[0-9]+[^/]*/schema/`).MatchString(lib) {
		t.Errorf("the library schema URL is not pinned to a release tag: %s", lib)
	}
	mod, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`ivcap-ncbi-genome-download (v[0-9]+\.[0-9]+\.[0-9]+)\s`).FindSubmatch(mod)
	if m == nil {
		t.Skip("the library is not on a release version (pseudo-version), nothing to compare")
	}
	if !strings.Contains(lib, "/"+string(m[1])+"/") {
		t.Errorf("go.mod uses the library at %s but its schema URL names another tag: %s", m[1], lib)
	}
}

func TestManifestSchemaMatchesCode(t *testing.T) {
	doc := readSchemaFile(t, "artifact-archive-manifest.1.schema.json")
	runSchemaChecks(t, doc, []schemaCheck{
		{"$id", []any{"$id"}, artifactArchiveManifestSchema},
		{"$schema const", []any{"properties", "$schema", "const"}, artifactArchiveManifestSchema},
		{"profile const", []any{"properties", "profile", "const"}, roCrateProfile},
		{"RO-Crate @context const", []any{"$defs", "roCrate", "properties", "@context", "const"}, roCrateContext},
		{"descriptor conformsTo", []any{"$defs", "roCrate", "properties", "@graph", "contains", "properties", "conformsTo", "properties", "@id", "const"}, roCrateProfile},
	})
}
