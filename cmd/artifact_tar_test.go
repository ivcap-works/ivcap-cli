// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsTarFile(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		data     []byte
		expected bool
	}{
		{
			name:     "tar mime type",
			mimeType: "application/x-tar",
			data:     []byte{},
			expected: true,
		},
		{
			name:     "gzip mime type",
			mimeType: "application/gzip",
			data:     []byte{},
			expected: true,
		},
		{
			name:     "tgz mime type",
			mimeType: "application/x-tgz",
			data:     []byte{},
			expected: true,
		},
		{
			name:     "gzip magic bytes",
			mimeType: "application/octet-stream",
			data:     []byte{0x1f, 0x8b, 0x08},
			expected: true,
		},
		{
			name:     "not a tar file",
			mimeType: "text/plain",
			data:     []byte("hello world"),
			expected: false,
		},
		{
			name:     "empty data",
			mimeType: "application/octet-stream",
			data:     []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTarFile(tt.mimeType, tt.data)
			if result != tt.expected {
				t.Errorf("isTarFile(%q, %v) = %v, want %v", tt.mimeType, tt.data, result, tt.expected)
			}
		})
	}
}

func TestNormalizeFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "file.txt",
			expected: "file.txt",
		},
		{
			name:     "path with directory",
			input:    "dir/file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "path with leading slash",
			input:    "/dir/file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "path with leading ./",
			input:    "./dir/file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "nested path",
			input:    "a/b/c/file.txt",
			expected: "a/b/c/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeFilePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestListTarFiles(t *testing.T) {
	// Create a test tar archive
	tarData := createTestTar(t, false)

	files, err := listTarFiles(tarData)
	if err != nil {
		t.Fatalf("listTarFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Check file names
	expectedNames := map[string]bool{
		"file1.txt":     false,
		"dir/file2.txt": false,
	}

	for _, f := range files {
		if _, ok := expectedNames[f.Name]; ok {
			expectedNames[f.Name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected file %q not found in listing", name)
		}
	}
}

func TestListTarFilesGzip(t *testing.T) {
	// Create a test tar.gz archive
	tarData := createTestTar(t, true)

	files, err := listTarFiles(tarData)
	if err != nil {
		t.Fatalf("listTarFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestExtractFileFromTar(t *testing.T) {
	tarData := createTestTar(t, false)

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		expected    string
	}{
		{
			name:        "extract existing file",
			filePath:    "file1.txt",
			expectError: false,
			expected:    "content1",
		},
		{
			name:        "extract file in directory",
			filePath:    "dir/file2.txt",
			expectError: false,
			expected:    "content2",
		},
		{
			name:        "extract with leading ./",
			filePath:    "./file1.txt",
			expectError: false,
			expected:    "content1",
		},
		{
			name:        "non-existent file",
			filePath:    "nonexistent.txt",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := extractFileFromTar(tarData, tt.filePath)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("extractFileFromTar() error = %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("extractFileFromTar() = %q, want %q", string(data), tt.expected)
			}
		})
	}
}

func TestExtractFileFromTarGzip(t *testing.T) {
	tarData := createTestTar(t, true)

	data, err := extractFileFromTar(tarData, "file1.txt")
	if err != nil {
		t.Fatalf("extractFileFromTar() error = %v", err)
	}
	if string(data) != "content1" {
		t.Errorf("extractFileFromTar() = %q, want %q", string(data), "content1")
	}
}

func TestSaveAndLoadTarCache(t *testing.T) {
	// Use a temporary directory for cache
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()
	os.Setenv("HOME", tmpDir)

	artifactID := "urn:ivcap:artifact:test-123"
	testData := []byte("test cache data")

	// Test save
	err := saveTarCache(artifactID, testData)
	if err != nil {
		t.Fatalf("saveTarCache() error = %v", err)
	}

	// Test load
	loaded, found := loadTarCache(artifactID)
	if !found {
		t.Errorf("loadTarCache() not found")
	}
	if !bytes.Equal(loaded, testData) {
		t.Errorf("loadTarCache() = %v, want %v", loaded, testData)
	}

	// Test load non-existent
	_, found = loadTarCache("urn:ivcap:artifact:nonexistent")
	if found {
		t.Errorf("loadTarCache() found non-existent artifact")
	}
}

func TestGetTarCacheDir(t *testing.T) {
	dir, err := getTarCacheDir()
	if err != nil {
		t.Fatalf("getTarCacheDir() error = %v", err)
	}

	// Should be an absolute path
	if !filepath.IsAbs(dir) {
		t.Errorf("getTarCacheDir() should return absolute path, got %q", dir)
	}

	// Should end with .ivcap/tar-cache
	expectedSuffix := filepath.Join(".ivcap", "tar-cache")
	if !strings.HasSuffix(dir, expectedSuffix) {
		t.Errorf("getTarCacheDir() should end with %q, got %q", expectedSuffix, dir)
	}
}

// Helper function to create a test tar archive
func createTestTar(t *testing.T, compress bool) []byte {
	t.Helper()

	buf := new(bytes.Buffer)
	var tw *tar.Writer

	if compress {
		gw := gzip.NewWriter(buf)
		defer gw.Close()
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(buf)
	}
	defer tw.Close()

	// Add test files
	files := []struct {
		name    string
		content string
	}{
		{"file1.txt", "content1"},
		{"dir/file2.txt", "content2"},
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.name,
			Mode: 0644,
			Size: int64(len(f.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(f.content)); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}

	if compress {
		tw.Close()
	}

	return buf.Bytes()
}
