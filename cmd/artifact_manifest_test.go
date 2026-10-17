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

	"github.com/spf13/cobra"
)

func TestArchiveManifestForFile(t *testing.T) {
	files := map[string]string{"dir/a file.txt": "alpha", "data/sample.csv": "x,y\n1,2\n"}
	for _, format := range []string{"zip", "tar", "tar.gz"} {
		t.Run(format, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "archive."+format)
			writeManifestTestArchive(t, archive, format, files)
			content, err := archiveManifestForFile(archive, artifactArchiveManifestSchema, archiveManifestMetadata{
				Identifier: "test-archive", Name: "Test archive", Published: "2026-10-17",
			})
			if err != nil {
				t.Fatal(err)
			}
			var aspect archiveManifestAspect
			if err := json.Unmarshal(content, &aspect); err != nil {
				t.Fatal(err)
			}
			if aspect.Schema != artifactArchiveManifestSchema || aspect.Profile != roCrateProfile || aspect.ROCrate.Context != roCrateContext {
				t.Errorf("unexpected manifest envelope: %+v", aspect)
			}
			if len(aspect.ROCrate.Graph) != len(files)+2 {
				t.Fatalf("graph has %d entities, want %d", len(aspect.ROCrate.Graph), len(files)+2)
			}
			root := aspect.ROCrate.Graph[1]
			if root.ID != "./" || root.Type != "Dataset" || root.DatePublished != "2026-10-17" || len(root.HasPart) != len(files) {
				t.Errorf("unexpected root entity: %+v", root)
			}
			var escaped bool
			for _, entity := range aspect.ROCrate.Graph[2:] {
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
	if _, err := archiveManifestForFile("-", artifactArchiveManifestSchema, archiveManifestMetadata{}); err == nil || !strings.Contains(err.Error(), "local archive") {
		t.Errorf("stdin must be rejected, got %v", err)
	}
	plain := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(plain, []byte("not an archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveManifestForFile(plain, artifactArchiveManifestSchema, archiveManifestMetadata{}); err == nil || !strings.Contains(err.Error(), "not a supported") {
		t.Errorf("plain file must be rejected, got %v", err)
	}

	unsafe := filepath.Join(t.TempDir(), "unsafe.zip")
	writeManifestTestArchive(t, unsafe, "zip", map[string]string{"../escape.txt": "bad"})
	if _, err := archiveManifestForFile(unsafe, artifactArchiveManifestSchema, archiveManifestMetadata{}); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("unsafe member must be rejected, got %v", err)
	}
	if _, err := safeArchivePath(`C:\escape.txt`); err == nil {
		t.Error("Windows absolute member path must be rejected")
	}
}

func TestPrepareArtifactManifestDefaults(t *testing.T) {
	oldWithout, oldSchema, oldName := artifactWithoutManifest, artifactManifestSchema, name
	t.Cleanup(func() {
		artifactWithoutManifest, artifactManifestSchema, name = oldWithout, oldSchema, oldName
	})
	artifactWithoutManifest = false
	artifactManifestSchema = artifactArchiveManifestSchema
	name = ""

	archive := filepath.Join(t.TempDir(), "archive.zip")
	writeManifestTestArchive(t, archive, "zip", map[string]string{"file.txt": "content"})
	content, err := prepareArtifactManifest(archive)
	if err != nil || len(content) == 0 {
		t.Fatalf("archive should get a manifest by default: len=%d err=%v", len(content), err)
	}

	artifactWithoutManifest = true
	content, err = prepareArtifactManifest(archive)
	if err != nil || content != nil {
		t.Fatalf("--without-manifest should opt out: content=%q err=%v", content, err)
	}
	artifactWithoutManifest = false

	plain := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(plain, []byte("plain"), 0600); err != nil {
		t.Fatal(err)
	}
	content, err = prepareArtifactManifest(plain)
	if err != nil || content != nil {
		t.Fatalf("ordinary files should upload without a manifest: content=%q err=%v", content, err)
	}
	content, err = prepareArtifactManifest("-")
	if err != nil || content != nil {
		t.Fatalf("stdin should upload without a manifest: content=%q err=%v", content, err)
	}

	extensionless := filepath.Join(t.TempDir(), "archive-data")
	writeManifestTestArchive(t, extensionless, "tar", map[string]string{"file.txt": "content"})
	content, err = prepareArtifactManifest(extensionless)
	if err != nil || len(content) == 0 {
		t.Fatalf("extensionless TAR should be detected: len=%d err=%v", len(content), err)
	}

	broken := filepath.Join(t.TempDir(), "broken.zip")
	if err := os.WriteFile(broken, []byte("not a zip"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareArtifactManifest(broken); err == nil || !strings.Contains(err.Error(), "not a supported") {
		t.Errorf("malformed claimed archive must fail, got %v", err)
	}
}

func TestArtifactManifestCommandWiring(t *testing.T) {
	for _, command := range []*cobra.Command{createArtifactCmd, uploadArtifactCmd} {
		if command.Flags().Lookup("without-manifest") == nil {
			t.Errorf("artifact %s lacks --without-manifest", command.Name())
		}
		flag := command.Flags().Lookup("manifest-schema")
		if flag == nil || flag.DefValue != artifactArchiveManifestSchema {
			t.Errorf("artifact %s has unexpected --manifest-schema: %v", command.Name(), flag)
		}
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
