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
	"github.com/ivcap-works/ivcap-cli/pkg/artifact"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGenomeLicense(t *testing.T) {
	for _, empty := range []string{"", "  "} {
		got, err := resolveGenomeLicense(empty)
		if err != nil || got != ncbiDataLicense {
			t.Errorf("%q: want the NCBI policy, got %+v (%v)", empty, got, err)
		}
	}
	if !strings.HasPrefix(ncbiDataLicense.ID, "https://www.ncbi.nlm.nih.gov/") || ncbiDataLicense.Name == "" ||
		!strings.Contains(ncbiDataLicense.Description, "not a license grant") {
		t.Errorf("the NCBI policy entity must point at NCBI and say that it grants no license: %+v", ncbiDataLicense)
	}
	if got, err := resolveGenomeLicense("undefined"); err != nil || got.ID != "#license-undefined" {
		t.Errorf("an explicit 'undefined' must win over the NCBI default: %+v (%v)", got, err)
	}
	if got, err := resolveGenomeLicense("CC0-1.0"); err != nil || got.ID != "https://spdx.org/licenses/CC0-1.0" {
		t.Errorf("an explicit license must win over the NCBI default: %+v (%v)", got, err)
	}
	if _, err := resolveGenomeLicense("not a license!"); err == nil {
		t.Error("an invalid license must be rejected")
	}
}

func TestPrepareArtifactManifestCarriesLicense(t *testing.T) {
	oldWithout, oldSchema, oldName, oldLicense := artifactWithoutManifest, artifactManifestSchema, name, artifactLicense
	t.Cleanup(func() {
		artifactWithoutManifest, artifactManifestSchema, name, artifactLicense = oldWithout, oldSchema, oldName, oldLicense
	})
	artifactWithoutManifest, artifactManifestSchema, name = false, artifact.ArchiveManifestSchema, ""

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
		var aspect artifact.Aspect
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
		if f == nil || f.DefValue != artifact.LicenseUndefined {
			t.Errorf("artifact %s: --license must default to %q, got %+v", cmd, artifact.LicenseUndefined, f)
		}
	}
	// An empty import license means "the NCBI data policy".
	if f := genomeImportCmd.Flags().Lookup("license"); f == nil || f.DefValue != "" {
		t.Errorf("genome import: --license must default to empty, got %+v", f)
	}
}
