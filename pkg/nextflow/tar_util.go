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

package nextflow

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
)

func safeString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// SanitizeTarPath validates and normalizes a file path intended to be stored
// inside a tar archive.
//
// It ensures a stable unix-style relative path, disallows traversal and volume
// names, and rejects empty/invalid names.
func SanitizeTarPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty name")
	}
	// Ensure unix separators in tar.
	p = strings.ReplaceAll(p, "\\", "/")
	// path.Clean uses forward slashes.
	p = path.Clean(p)
	// Remove any leading slashes.
	p = strings.TrimPrefix(p, "/")
	// Ensure not empty and not traversal.
	if p == "." || p == "" {
		return "", fmt.Errorf("invalid tar path: %q", p)
	}
	if strings.HasPrefix(p, "../") || p == ".." {
		return "", fmt.Errorf("invalid tar path (traversal): %q", p)
	}
	// Windows drive letters, etc.
	if vol := filepath.VolumeName(p); vol != "" {
		return "", fmt.Errorf("invalid tar path (volume): %q", p)
	}
	// Disallow backtracking segments anywhere.
	if strings.Contains("/"+p+"/", "/../") {
		return "", fmt.Errorf("invalid tar path (traversal): %q", p)
	}
	return p, nil
}

// Backwards-compatibility for internal callers.
func sanitizeTarPath(p string) (string, error) {
	return SanitizeTarPath(p)
}

// ExtractFromTarAuto extracts a file from tar or tar.gz bytes.
func ExtractFromTarAuto(data []byte, innerPath string) ([]byte, string, error) {
	innerPath = strings.TrimPrefix(innerPath, "/")
	if innerPath == "" {
		return nil, "", fmt.Errorf("missing inner path")
	}

	// gz?
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = gzr.Close() }()
		b, _, err := ExtractFileFromTarReader(tar.NewReader(gzr), innerPath)
		return b, "application/octet-stream", err
	}
	b, _, err := ExtractFileFromTarReader(tar.NewReader(bytes.NewReader(data)), innerPath)
	return b, "application/octet-stream", err
}

// ExtractFromTarAuto is used for Source type=artifact when an inner path is provided.
// For now mime-type detection is best-effort and defaults to application/octet-stream.
func extractFromTarAuto(data []byte, innerPath string) ([]byte, string, error) {
	return ExtractFromTarAuto(data, innerPath)
}

// DrainTar is a helper used in tests/debugging.
func DrainTar(r io.Reader) ([]string, error) {
	tr := tar.NewReader(r)
	var names []string
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return names, nil
			}
			return nil, err
		}
		names = append(names, h.Name)
	}
}
