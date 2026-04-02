// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"
)

func TestSanitizeTarPath_AllowsDirs(t *testing.T) {
	p, err := sanitizeTarPath("data/contract.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "data/contract.pdf" {
		t.Fatalf("unexpected path: %q", p)
	}
}

func TestSanitizeTarPath_RejectsTraversal(t *testing.T) {
	_, err := sanitizeTarPath("../secret.txt")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTarGzFromParts_UsesNamesAsPaths(t *testing.T) {
	parts := []mcpContentPart{
		{Type: "text", Text: "hello", Name: "a/b.txt"},
		{Type: "text", Text: "world", Name: "c.txt"},
	}

	b, err := tarGzFromParts(context.Background(), parts)
	if err != nil {
		t.Fatalf("tarGzFromParts error: %v", err)
	}

	gzr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	seen := map[string]string{}
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tar read error: %v", err)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(tr)
		seen[h.Name] = buf.String()
	}

	if seen["a/b.txt"] != "hello" {
		t.Fatalf("missing/incorrect a/b.txt: %+v", seen)
	}
	if seen["c.txt"] != "world" {
		t.Fatalf("missing/incorrect c.txt: %+v", seen)
	}
	if _, ok := seen["MANIFEST.json"]; !ok {
		t.Fatalf("expected MANIFEST.json in tar")
	}
}
