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
	"github.com/ivcap-works/ivcap-cli/pkg/artifact"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrepareArtifactManifestDefaults(t *testing.T) {
	oldWithout, oldSchema, oldName := artifactWithoutManifest, artifactManifestSchema, name
	t.Cleanup(func() {
		artifactWithoutManifest, artifactManifestSchema, name = oldWithout, oldSchema, oldName
	})
	artifactWithoutManifest = false
	artifactManifestSchema = artifact.ArchiveManifestSchema
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

	// Only the file name decides: extensionless files and other ZIP-based formats get no manifest.
	for _, other := range []string{"archive-data", "report.docx", "library.jar"} {
		otherPath := filepath.Join(t.TempDir(), other)
		writeManifestTestArchive(t, otherPath, "zip", map[string]string{"file.txt": "content"})
		content, err = prepareArtifactManifest(otherPath)
		if err != nil || content != nil {
			t.Fatalf("%s must not get a manifest: content=%q err=%v", other, content, err)
		}
	}
	for archiveName, format := range map[string]string{"a.zip": "zip", "a.tar": "tar", "a.tar.gz": "tar.gz", "a.tgz": "tar.gz", "UPPER.ZIP": "zip"} {
		archivePath := filepath.Join(t.TempDir(), archiveName)
		writeManifestTestArchive(t, archivePath, format, map[string]string{"file.txt": "content"})
		content, err = prepareArtifactManifest(archivePath)
		if err != nil || len(content) == 0 {
			t.Fatalf("%s should get a manifest: len=%d err=%v", archiveName, len(content), err)
		}
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
		if flag == nil || flag.DefValue != artifact.ArchiveManifestSchema {
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

func TestManifestForUploadWarnsInsteadOfFailing(t *testing.T) {
	oldWithout, oldSchema, oldName := artifactWithoutManifest, artifactManifestSchema, name
	t.Cleanup(func() {
		artifactWithoutManifest, artifactManifestSchema, name = oldWithout, oldSchema, oldName
	})
	artifactWithoutManifest, artifactManifestSchema, name = false, artifact.ArchiveManifestSchema, ""

	dir := t.TempDir()
	good := filepath.Join(dir, "good.zip")
	writeManifestTestArchive(t, good, "zip", map[string]string{"file.txt": "content"})
	broken := filepath.Join(dir, "broken.tar.gz")
	if err := os.WriteFile(broken, []byte("not an archive"), 0600); err != nil {
		t.Fatal(err)
	}
	unsafe := filepath.Join(dir, "unsafe.zip")
	writeManifestTestArchive(t, unsafe, "zip", map[string]string{"../escape.txt": "bad"})
	plain := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(plain, []byte("plain"), 0600); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	if got := manifestForUpload(good, &warn); len(got) == 0 || warn.Len() != 0 {
		t.Errorf("a good archive must get a manifest and no warning: len=%d warning=%q", len(got), warn.String())
	}
	for _, bad := range []string{broken, unsafe} {
		warn.Reset()
		if got := manifestForUpload(bad, &warn); got != nil {
			t.Errorf("%s: no manifest expected, got %d bytes", filepath.Base(bad), len(got))
		}
		if !strings.Contains(warn.String(), "Warning:") || !strings.Contains(warn.String(), bad) {
			t.Errorf("%s: expected a warning naming the file, got %q", filepath.Base(bad), warn.String())
		}
	}
	warn.Reset()
	if got := manifestForUpload(plain, &warn); got != nil || warn.Len() != 0 {
		t.Errorf("an ordinary file needs neither a manifest nor a warning: %q %q", got, warn.String())
	}
}

func TestWarnManifestNotAttached(t *testing.T) {
	var warn bytes.Buffer
	warnManifestNotAttached(&warn, "urn:ivcap:artifact:abc", io.ErrUnexpectedEOF)
	for _, want := range []string{"Warning:", "urn:ivcap:artifact:abc", "unexpected EOF", "again"} {
		if !strings.Contains(warn.String(), want) {
			t.Errorf("warning lacks %q: %q", want, warn.String())
		}
	}
}
