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

package mcp

import (
	"testing"
)

func TestNormalizeInnerPath(t *testing.T) {
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
			name:     "UUID directory with file",
			input:    "9c1db38c-e180-4570-9874-e85db9bda90f/.nextflow.log",
			expected: "9c1db38c-e180-4570-9874-e85db9bda90f/.nextflow.log",
		},
		{
			name:     "UUID directory with ./ prefix",
			input:    "./9c1db38c-e180-4570-9874-e85db9bda90f/.nextflow.log",
			expected: "9c1db38c-e180-4570-9874-e85db9bda90f/.nextflow.log",
		},
		{
			name:     "UUID directory with / prefix",
			input:    "/9c1db38c-e180-4570-9874-e85db9bda90f/.nextflow.log",
			expected: "9c1db38c-e180-4570-9874-e85db9bda90f/.nextflow.log",
		},
		{
			name:     "just dot",
			input:    ".",
			expected: "",
		},
		{
			name:     "just double dot",
			input:    "..",
			expected: "",
		},
		{
			name:     "path traversal attempt",
			input:    "../etc/passwd",
			expected: "",
		},
		{
			name:     "path with embedded traversal",
			input:    "dir/../etc/passwd",
			expected: "",
		},
		{
			name:     "multiple slashes",
			input:    "dir//file.txt",
			expected: "dir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeInnerPath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeInnerPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "text file",
			filePath: "file.txt",
			expected: "text/plain",
		},
		{
			name:     "log file",
			filePath: ".nextflow.log",
			expected: "text/plain",
		},
		{
			name:     "JSON file",
			filePath: "config.json",
			expected: "application/json",
		},
		{
			name:     "YAML file",
			filePath: "config.yaml",
			expected: "application/yaml",
		},
		{
			name:     "YML file",
			filePath: "config.yml",
			expected: "application/yaml",
		},
		{
			name:     "Nextflow file",
			filePath: "main.nf",
			expected: "text/x-nextflow",
		},
		{
			name:     "shell script",
			filePath: "script.sh",
			expected: "application/x-sh",
		},
		{
			name:     "markdown",
			filePath: "README.md",
			expected: "text/markdown",
		},
		{
			name:     "HTML",
			filePath: "index.html",
			expected: "text/html",
		},
		{
			name:     "config file",
			filePath: "nextflow.config",
			expected: "text/plain",
		},
		{
			name:     "unknown extension",
			filePath: "file.unknown",
			expected: "application/octet-stream",
		},
		{
			name:     "no extension",
			filePath: "README",
			expected: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMimeType(tt.filePath, nil)
			if result != tt.expected {
				t.Errorf("detectMimeType(%q) = %q, want %q", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestShouldReturnAsText(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		accept   []string
		expected bool
	}{
		{
			name:     "no accept header returns text-compatible content as text by default",
			mimeType: "text/plain",
			accept:   nil,
			expected: true,
		},
		{
			name:     "empty accept array returns text-compatible content as text by default",
			mimeType: "text/plain",
			accept:   []string{},
			expected: true,
		},
		{
			name:     "exact match text/plain",
			mimeType: "text/plain",
			accept:   []string{"text/plain"},
			expected: true,
		},
		{
			name:     "wildcard text/*",
			mimeType: "text/plain",
			accept:   []string{"text/*"},
			expected: true,
		},
		{
			name:     "wildcard matches nextflow",
			mimeType: "text/x-nextflow",
			accept:   []string{"text/*"},
			expected: true,
		},
		{
			name:     "application/json with text/*",
			mimeType: "application/json",
			accept:   []string{"text/*"},
			expected: true,
		},
		{
			name:     "application/yaml with text/*",
			mimeType: "application/yaml",
			accept:   []string{"text/*"},
			expected: true,
		},
		{
			name:     "binary type with text/* should be false",
			mimeType: "application/octet-stream",
			accept:   []string{"text/*"},
			expected: false,
		},
		{
			name:     "image type with text/* should be false",
			mimeType: "image/png",
			accept:   []string{"text/*"},
			expected: false,
		},
		{
			name:     "multiple accept types with match",
			mimeType: "application/json",
			accept:   []string{"image/png", "application/json"},
			expected: true,
		},
		{
			name:     "case insensitive matching",
			mimeType: "text/plain",
			accept:   []string{"TEXT/PLAIN"},
			expected: true,
		},
		{
			name:     "case insensitive wildcard",
			mimeType: "text/plain",
			accept:   []string{"TEXT/*"},
			expected: true,
		},
		{
			name:     "*/* wildcard for text",
			mimeType: "text/plain",
			accept:   []string{"*/*"},
			expected: true,
		},
		{
			name:     "application/xml is text-compatible",
			mimeType: "application/xml",
			accept:   []string{"text/*"},
			expected: true,
		},
		{
			name:     "application/x-sh is text-compatible",
			mimeType: "application/x-sh",
			accept:   []string{"text/*"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldReturnAsText(tt.mimeType, tt.accept)
			if result != tt.expected {
				t.Errorf("shouldReturnAsText(%q, %v) = %v, want %v",
					tt.mimeType, tt.accept, result, tt.expected)
			}
		})
	}
}
