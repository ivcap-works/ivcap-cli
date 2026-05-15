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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

type artifactGetArgs struct {
	ID     string   `json:"id"`
	Path   string   `json:"path,omitempty"`
	Accept []string `json:"accept,omitempty"`
}

// A tiny in-process cache for the most recently accessed tar artifact.
// Artifacts are assumed to be immutable, so caching by ID is safe.
type tarArtifactCache struct {
	mu        sync.Mutex
	artifact  string
	raw       []byte
	mediaType string
}

var lastTarCache tarArtifactCache

func addArtifactGetTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Artifact URN/ID.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional path inside a tar/tar.gz artifact to return only that file.",
			},
			"accept": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "Optional array of acceptable content formats (like HTTP Accept header). Supported values: 'text/*', 'text/plain', 'application/json', etc. If omitted or if no text format matches, returns base64-encoded data. Use text/* formats to get raw text content instead of base64.",
			},
		},
		"required": []any{"id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"artifact_get",
		"Fetch an IVCAP artifact (URN format: urn:ivcap:artifact:...). If `path` is provided and the artifact is a tar/tar.gz, extracts and returns only that specific file from the archive. NOTE: This tool is for IVCAP artifacts only. For embedded skill documentation (skills://...), use MCP's resources/read method instead.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed artifactGetArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.ID == "" {
			return nil, fmt.Errorf("missing id")
		}

		// Detect skills:// URLs and provide helpful error message
		if strings.HasPrefix(parsed.ID, "skills://") {
			return nil, fmt.Errorf("skills:// URLs are MCP resources, not IVCAP artifacts. Use MCP's resources/read method instead. Available skill resources are documented in skills://manifest")
		}

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		art, err := readArtifactFn(ctxt, &sdk.ReadArtifactRequest{Id: parsed.ID}, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		if art == nil || art.DataHref == nil {
			return nil, fmt.Errorf("artifact has no data")
		}
		mimeType := safeString(art.MimeType)
		dataURL := *art.DataHref

		// If caller asked for an internal tar path, attempt tar extraction.
		if parsed.Path != "" {
			b, _, err := getTarArtifactBytesCached(ctxt, parsed.ID, dataURL, mimeType, adpt)
			if err != nil {
				return nil, err
			}
			if !looksLikeTar(mimeType, b) {
				return nil, fmt.Errorf("artifact %q does not look like a tar/tar.gz, cannot use path", parsed.ID)
			}
			inner, innerMime, err := extractFromTarAuto(b, parsed.Path)
			if err != nil {
				return nil, err
			}
			return toolResultBlob(inner, innerMime, parsed.Accept)
		}

		// Otherwise, return entire artifact.
		data, err := downloadArtifactBytesFn(ctxt, dataURL, adpt)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		// Populate cache if it's a tar-ish type
		maybeUpdateTarCache(parsed.ID, data, mimeType)
		return toolResultBlob(data, mimeType, parsed.Accept)
	}

	s.AddTool(tool, handler)
}

func toolResultBlob(b []byte, mimeType string, accept []string) (*mcp.CallToolResult, error) {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Check if caller accepts text format and content is text-compatible
	if shouldReturnAsText(mimeType, accept) {
		// For text content: return plain text directly (no StructuredContent to avoid confusion)
		// Clients get the actual content in Content field, not buried in metadata
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(string(b))},
		}, nil
	}

	// Default: return as base64-encoded blob using BlobResource
	// This clearly indicates the content is binary/encoded data
	enc := base64.StdEncoding.EncodeToString(b)
	res := mcp.BlobResourceContents{
		URI:      "urn:ivcap:artifact:data",
		MIMEType: mimeType,
		Blob:     enc,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewEmbeddedResource(res)},
	}, nil
}

// shouldReturnAsText determines if content should be returned as text based on
// the content's MIME type and the caller's accept preferences.
// By default, text-compatible content is returned as text unless accept is
// explicitly provided with binary-only types.
func shouldReturnAsText(mimeType string, accept []string) bool {
	// Check if mimeType is text-compatible
	isTextType := strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml" ||
		mimeType == "application/yaml" ||
		mimeType == "application/x-sh" ||
		strings.HasSuffix(mimeType, "+xml") ||
		strings.HasSuffix(mimeType, "+json")

	if !isTextType {
		return false // Binary content, must use base64
	}

	// If no accept preference is specified, return text-compatible content as text by default
	if len(accept) == 0 {
		return true
	}

	// Check if any accept type matches a text format
	for _, acceptType := range accept {
		acceptType = strings.TrimSpace(strings.ToLower(acceptType))
		mimeTypeLower := strings.ToLower(mimeType)

		// Exact match
		if acceptType == mimeTypeLower {
			return true
		}

		// Wildcard match (e.g., "text/*")
		if strings.HasSuffix(acceptType, "/*") {
			prefix := strings.TrimSuffix(acceptType, "/*")
			if strings.HasPrefix(mimeTypeLower, prefix+"/") {
				return true
			}
		}

		// Generic text wildcard
		if acceptType == "*/*" || acceptType == "text/*" {
			return true
		}

		// Special case: CSV and TSV can be returned as text/plain since they are plain text
		if acceptType == "text/plain" {
			if mimeType == "text/csv" || mimeType == "text/tsv" {
				return true
			}
		}
	}

	return false
}

func getTarArtifactBytesCached(ctx context.Context, artifactID, dataHref, mimeType string, adpt *a.Adapter) ([]byte, string, error) {
	lastTarCache.mu.Lock()
	if lastTarCache.artifact == artifactID && lastTarCache.raw != nil {
		b := lastTarCache.raw
		mt := lastTarCache.mediaType
		lastTarCache.mu.Unlock()
		return b, mt, nil
	}
	lastTarCache.mu.Unlock()

	data, err := downloadArtifactBytesFn(ctx, dataHref, adpt)
	if err != nil {
		return nil, "", err
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	maybeUpdateTarCache(artifactID, data, mimeType)
	return data, mimeType, nil
}

func maybeUpdateTarCache(artifactID string, data []byte, mimeType string) {
	if !looksLikeTar(mimeType, data) {
		return
	}
	lastTarCache.mu.Lock()
	defer lastTarCache.mu.Unlock()
	lastTarCache.artifact = artifactID
	lastTarCache.raw = data
	lastTarCache.mediaType = mimeType
}

func looksLikeTar(mimeType string, data []byte) bool {
	mt := strings.ToLower(mimeType)
	if strings.Contains(mt, "tar") || strings.Contains(mt, "gzip") || strings.Contains(mt, "tgz") {
		return true
	}
	// Heuristic: gzip magic
	return len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b
}

func extractFromTarAuto(data []byte, innerPath string) ([]byte, string, error) {
	// Normalize the requested path - handle both "path" and "./path" variations
	innerPath = normalizeInnerPath(innerPath)
	if innerPath == "" {
		return nil, "", fmt.Errorf("invalid inner path: empty after normalization")
	}

	// Try gzip first.
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = gzr.Close() }()
		return extractFromTarReader(tar.NewReader(gzr), innerPath)
	}
	return extractFromTarReader(tar.NewReader(bytes.NewReader(data)), innerPath)
}

// normalizeInnerPath normalizes a path for tar file matching.
// It handles variations like "path", "./path", "/path" and returns a clean path.
// Returns empty string for invalid paths (. or paths with ..).
func normalizeInnerPath(p string) string {
	// Check for path traversal attempts BEFORE cleaning
	// (path.Clean would resolve them away)
	if strings.Contains(p, "..") {
		return ""
	}

	// Remove leading slashes and "./" prefix
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "./")

	// Clean the path
	p = path.Clean(p)

	// Reject if it resolved to just "."
	if p == "." {
		return ""
	}

	return p
}

func extractFromTarReader(tr *tar.Reader, innerPath string) ([]byte, string, error) {
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil, "", fmt.Errorf("file %q not found in tar", innerPath)
			}
			return nil, "", err
		}
		if h == nil {
			continue
		}
		// Normalize the tar entry name the same way we normalize the search path
		name := normalizeInnerPath(h.Name)
		if name == innerPath {
			b, err := io.ReadAll(tr)
			if err != nil {
				return nil, "", err
			}
			// Try to detect content type from file extension
			mimeType := detectMimeType(innerPath, b)
			return b, mimeType, nil
		}
	}
}

// detectMimeType attempts to detect MIME type from file path and content.
func detectMimeType(filePath string, data []byte) string {
	// Simple extension-based detection
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".txt", ".log":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".html", ".htm":
		return "text/html"
	case ".md":
		return "text/markdown"
	case ".sh":
		return "application/x-sh"
	case ".nf":
		return "text/x-nextflow"
	case ".config":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".tsv":
		return "text/tsv"
	default:
		return "application/octet-stream"
	}
}
