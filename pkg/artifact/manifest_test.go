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

package artifact

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveManifestForFile(t *testing.T) {
	files := map[string]string{"dir/a file.txt": "alpha", "data/sample.csv": "x,y\n1,2\n"}
	for _, format := range []string{"zip", "tar", "tar.gz"} {
		t.Run(format, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "archive."+format)
			writeManifestTestArchive(t, archive, format, files)
			content, err := ForFile(archive, ArchiveManifestSchema, Metadata{
				Identifier: "test-archive", Name: "Test archive", Published: "2026-10-17",
			})
			if err != nil {
				t.Fatal(err)
			}
			var aspect Aspect
			if err := json.Unmarshal(content, &aspect); err != nil {
				t.Fatal(err)
			}
			if aspect.Schema != ArchiveManifestSchema || aspect.Profile != ROCrateProfile || aspect.ROCrate.Context != ROCrateContext {
				t.Errorf("unexpected manifest envelope: %+v", aspect)
			}
			if len(aspect.ROCrate.Graph) != len(files)+3 { // descriptor, root, files, license
				t.Fatalf("graph has %d entities, want %d", len(aspect.ROCrate.Graph), len(files)+3)
			}
			root := aspect.ROCrate.Graph[1]
			if root.ID != "./" || root.Type != "Dataset" || root.DatePublished != "2026-10-17" || len(root.HasPart) != len(files) {
				t.Errorf("unexpected root entity: %+v", root)
			}
			var escaped bool
			for _, entity := range aspect.ROCrate.Graph[2 : 2+len(files)] {
				if entity.Type != "File" || entity.ContentSize == "" || entity.EncodingFormat == "" {
					t.Errorf("incomplete file entity: %+v", entity)
				}
				escaped = escaped || strings.Contains(entity.ID, "%20")
			}
			if !escaped {
				t.Error("file paths containing spaces were not URI-escaped")
			}
		})
	}
}

func TestArchiveManifestRejectsUnsupportedAndUnsafeInputs(t *testing.T) {
	if _, err := ForFile("-", ArchiveManifestSchema, Metadata{}); err == nil || !strings.Contains(err.Error(), "local archive") {
		t.Errorf("stdin must be rejected, got %v", err)
	}
	plain := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(plain, []byte("not an archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ForFile(plain, ArchiveManifestSchema, Metadata{}); err == nil || !strings.Contains(err.Error(), "not a supported") {
		t.Errorf("plain file must be rejected, got %v", err)
	}

	unsafe := filepath.Join(t.TempDir(), "unsafe.zip")
	writeManifestTestArchive(t, unsafe, "zip", map[string]string{"../escape.txt": "bad"})
	if _, err := ForFile(unsafe, ArchiveManifestSchema, Metadata{}); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("unsafe member must be rejected, got %v", err)
	}
	if _, err := safeArchivePath(`C:\escape.txt`); err == nil {
		t.Error("Windows absolute member path must be rejected")
	}
}

func writeManifestTestArchive(t *testing.T, archive, format string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	switch format {
	case "zip":
		zw := zip.NewWriter(&buf)
		for name, content := range files {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
	case "tar", "tar.gz":
		var out io.Writer = &buf
		var gz *gzip.Writer
		if format == "tar.gz" {
			gz = gzip.NewWriter(&buf)
			out = gz
		}
		tw := tar.NewWriter(out)
		for name, content := range files {
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(content))}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if gz != nil {
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
		}
	default:
		t.Fatalf("unsupported test archive format %q", format)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

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
		got, err := ResolveLicense(tt.value)
		if err != nil {
			t.Errorf("%q: unexpected error %v", tt.value, err)
			continue
		}
		if got.ID != tt.wantID || got.Name != tt.wantName || got.Description == "" {
			t.Errorf("%q: got %+v, want id %q and name %q with a description", tt.value, got, tt.wantID, tt.wantName)
		}
	}
	for _, bad := range []string{"CC BY 4.0", "not a license!", "http://", "https:///only-a-path", "ftp://example.org/l", "-MIT"} {
		if _, err := ResolveLicense(bad); err == nil || !strings.Contains(err.Error(), "invalid --license") {
			t.Errorf("%q: want an 'invalid --license' error, got %v", bad, err)
		}
	}
}

func TestBuildROCrateKeepsPathsUniqueAndSafe(t *testing.T) {
	doc := Build(Metadata{Name: "n"}, []Entry{
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
