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
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLicense(t *testing.T) {
	tests := []struct {
		value, wantID, wantName string
	}{
		{"", "#license-undefined", "License undefined"},
		{"undefined", "#license-undefined", "License undefined"},
		{"UNDEFINED", "#license-undefined", "License undefined"},
		{"  undefined ", "#license-undefined", "License undefined"},
		{"CC-BY-4.0", "https://spdx.org/licenses/CC-BY-4.0", "CC-BY-4.0"},
		{"MIT", "https://spdx.org/licenses/MIT", "MIT"},
		{"Apache-2.0", "https://spdx.org/licenses/Apache-2.0", "Apache-2.0"},
		{"https://creativecommons.org/licenses/by/4.0/", "https://creativecommons.org/licenses/by/4.0/", "https://creativecommons.org/licenses/by/4.0/"},
		{" http://example.org/license ", "http://example.org/license", "http://example.org/license"},
	}
	for _, tt := range tests {
		got, err := resolveLicense(tt.value)
		if err != nil {
			t.Errorf("%q: unexpected error %v", tt.value, err)
			continue
		}
		if got.ID != tt.wantID || got.Name != tt.wantName || got.Description == "" {
			t.Errorf("%q: got %+v, want id %q and name %q with a description", tt.value, got, tt.wantID, tt.wantName)
		}
	}
	for _, bad := range []string{"CC BY 4.0", "not a license!", "http://", "https:///only-a-path", "ftp://example.org/l", "-MIT"} {
		if _, err := resolveLicense(bad); err == nil || !strings.Contains(err.Error(), "invalid --license") {
			t.Errorf("%q: want an 'invalid --license' error, got %v", bad, err)
		}
	}
}

func TestResolveNcbiLicense(t *testing.T) {
	for _, empty := range []string{"", "  "} {
		got, err := resolveNcbiLicense(empty)
		if err != nil || got != ncbiDataLicense {
			t.Errorf("%q: want the NCBI policy, got %+v (%v)", empty, got, err)
		}
	}
	if !strings.HasPrefix(ncbiDataLicense.ID, "https://www.ncbi.nlm.nih.gov/") || ncbiDataLicense.Name == "" ||
		!strings.Contains(ncbiDataLicense.Description, "not a license grant") {
		t.Errorf("the NCBI policy entity must point at NCBI and say that it grants no license: %+v", ncbiDataLicense)
	}
	if got, err := resolveNcbiLicense("undefined"); err != nil || got.ID != "#license-undefined" {
		t.Errorf("an explicit 'undefined' must win over the NCBI default: %+v (%v)", got, err)
	}
	if got, err := resolveNcbiLicense("CC0-1.0"); err != nil || got.ID != "https://spdx.org/licenses/CC0-1.0" {
		t.Errorf("an explicit license must win over the NCBI default: %+v (%v)", got, err)
	}
	if _, err := resolveNcbiLicense("not a license!"); err == nil {
		t.Error("an invalid license must be rejected")
	}
}

func TestPrepareArtifactManifestCarriesLicense(t *testing.T) {
	oldWithout, oldSchema, oldName, oldLicense := artifactWithoutManifest, artifactManifestSchema, name, artifactLicense
	t.Cleanup(func() {
		artifactWithoutManifest, artifactManifestSchema, name, artifactLicense = oldWithout, oldSchema, oldName, oldLicense
	})
	artifactWithoutManifest, artifactManifestSchema, name = false, artifactArchiveManifestSchema, ""

	archive := filepath.Join(t.TempDir(), "data.zip")
	writeManifestTestArchive(t, archive, "zip", map[string]string{"file.txt": "content"})

	for value, wantID := range map[string]string{
		"undefined":                    "#license-undefined",
		"CC-BY-4.0":                    "https://spdx.org/licenses/CC-BY-4.0",
		"https://example.org/licence/": "https://example.org/licence/",
	} {
		artifactLicense = value
		content, err := prepareArtifactManifest(archive)
		if err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		var aspect archiveManifestAspect
		if err := json.Unmarshal(content, &aspect); err != nil {
			t.Fatal(err)
		}
		graph := aspect.ROCrate.Graph
		root, last := graph[1], graph[len(graph)-1]
		if root.License == nil || root.License.ID != wantID {
			t.Errorf("%q: the root dataset must point at %q, got %+v", value, wantID, root.License)
		}
		if last.ID != wantID || last.Type != "CreativeWork" || last.Name == "" || last.Description == "" {
			t.Errorf("%q: the last entity must describe the license %q, got %+v", value, wantID, last)
		}
		var withThatID int
		for _, entity := range graph {
			if entity.ID == wantID {
				withThatID++
			}
		}
		if withThatID != 1 {
			t.Errorf("%q: %d entities have the id %q, want exactly one", value, withThatID, wantID)
		}
	}

	artifactLicense = "not a license!"
	if _, err := prepareArtifactManifest(archive); err == nil || !strings.Contains(err.Error(), "invalid --license") {
		t.Errorf("an invalid license must be an error, got %v", err)
	}
}

func TestLicenseFlagDefaults(t *testing.T) {
	for _, cmd := range []string{"create", "upload"} {
		c := createArtifactCmd
		if cmd == "upload" {
			c = uploadArtifactCmd
		}
		f := c.Flags().Lookup("license")
		if f == nil || f.DefValue != licenseUndefined {
			t.Errorf("artifact %s: --license must default to %q, got %+v", cmd, licenseUndefined, f)
		}
	}
	// An empty import license means "the NCBI data policy".
	if f := ncbiImportCmd.Flags().Lookup("license"); f == nil || f.DefValue != "" {
		t.Errorf("ncbi genome import: --license must default to empty, got %+v", f)
	}
}

func TestBuildROCrateKeepsPathsUniqueAndSafe(t *testing.T) {
	doc := buildROCrate(archiveManifestMetadata{Name: "n"}, []archiveManifestEntry{
		{Name: "a.txt", Size: 1},
		{Name: "dir/b.txt", Size: 2},
		{Name: "a.txt", Size: 3}, // the same path again: the later entry wins
		{Name: "c:d/e.txt", Size: 4},
		{Name: "x:y", Size: 5},
		{Name: "dir/with:colon.txt", Size: 6}, // a colon after the first segment is harmless
	})
	graph := doc.Graph
	if len(graph) != 8 { // descriptor, root, five files, license
		t.Fatalf("graph has %d entities, want 8", len(graph))
	}
	seen := map[string]bool{}
	for _, e := range graph {
		if seen[e.ID] {
			t.Errorf("duplicate @id %q", e.ID)
		}
		seen[e.ID] = true
	}
	files := graph[2 : len(graph)-1]
	wantIDs := []string{"a.txt", "dir/b.txt", "./c:d/e.txt", "./x:y", "dir/with:colon.txt"}
	root := graph[1]
	for i, want := range wantIDs {
		if files[i].ID != want {
			t.Errorf("file %d has @id %q, want %q", i, files[i].ID, want)
		}
		if root.HasPart[i].ID != want {
			t.Errorf("hasPart %d is %q, want %q", i, root.HasPart[i].ID, want)
		}
		if first, _, _ := strings.Cut(files[i].ID, "/"); strings.Contains(first, ":") {
			t.Errorf("@id %q starts with a segment that reads as a URI scheme", files[i].ID)
		}
	}
	if files[0].ContentSize != "3" {
		t.Errorf("a repeated path must keep the size of its last entry, got %q", files[0].ContentSize)
	}
	if len(root.HasPart) != len(files) {
		t.Errorf("root lists %d parts for %d files", len(root.HasPart), len(files))
	}
}
