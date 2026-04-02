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

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

func makeTarGz(entries map[string][]byte) []byte {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, data := range entries {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))})
		_, _ = tw.Write(data)
	}
	_ = tw.Close()
	_ = gzw.Close()
	return buf.Bytes()
}

func TestExtractFromTarAuto_Gz(t *testing.T) {
	tarball := makeTarGz(map[string][]byte{"a/b.txt": []byte("hello")})
	b, _, err := extractFromTarAuto(tarball, "a/b.txt")
	if err != nil {
		t.Fatalf("extractFromTarAuto: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("unexpected content: %q", string(b))
	}
}

func TestTarCache_ServesSecondPathWithoutRedownload(t *testing.T) {
	// Reset global cache
	lastTarCache = tarArtifactCache{}
	defer func(old func(context.Context, string, *a.Adapter) ([]byte, error)) {
		downloadArtifactBytesFn = old
	}(downloadArtifactBytesFn)

	tarball := makeTarGz(map[string][]byte{
		"p1.txt": []byte("one"),
		"p2.txt": []byte("two"),
	})
	calls := 0
	downloadArtifactBytesFn = func(_ context.Context, _ string, _ *a.Adapter) ([]byte, error) {
		calls++
		return tarball, nil
	}

	ctx := context.Background()
	adpt := (*a.Adapter)(nil)
	_, _, err := getTarArtifactBytesCached(ctx, "urn:ivcap:artifact:1", "http://x", "application/gzip", adpt)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	// Second access should hit cache
	_, _, err = getTarArtifactBytesCached(ctx, "urn:ivcap:artifact:1", "http://x", "application/gzip", adpt)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 download call, got %d", calls)
	}

	// Also validate extraction works for both paths.
	data, _, _ := getTarArtifactBytesCached(ctx, "urn:ivcap:artifact:1", "http://x", "application/gzip", adpt)
	got1, _, err := extractFromTarAuto(data, "p1.txt")
	if err != nil || string(got1) != "one" {
		t.Fatalf("extract p1: %v %q", err, string(got1))
	}
	got2, _, err := extractFromTarAuto(data, "p2.txt")
	if err != nil || string(got2) != "two" {
		t.Fatalf("extract p2: %v %q", err, string(got2))
	}
}

func TestExtractFromTarReader_EOFHandling(t *testing.T) {
	// Ensure we don’t treat EOF as success when file is missing.
	tarball := makeTarGz(map[string][]byte{"x.txt": []byte("x")})
	_, _, err := extractFromTarAuto(tarball, "missing.txt")
	if err == nil {
		t.Fatalf("expected error")
	}
	if err == io.EOF {
		t.Fatalf("should not leak EOF")
	}
}
