package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// This file provides a built-in MCP tool to create an IVCAP artifact from
// LLM-style `content[]` parts.

type mcpContentPart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Name    string `json:"name,omitempty"`
	Title   string `json:"title,omitempty"`
	Context string `json:"context,omitempty"`
	Source  *struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data,omitempty"`
		URL       string `json:"url,omitempty"`
	} `json:"source,omitempty"`
}

// allow test stubbing
var fetchURLBytesFn = fetchURLBytes

type artifactCreateArgs struct {
	// Optional display name stored in the artifact record
	Name string `json:"name,omitempty"`
	// Optional collection id/urn
	Collection string `json:"collection,omitempty"`
	// Optional policy
	Policy string `json:"policy,omitempty"`

	// Content parts. If there is exactly 1 part, it becomes the artifact content.
	// If there are multiple parts, they are packaged into a single tar.gz.
	Content []mcpContentPart `json:"content"`
}

func addArtifactCreateTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Optional artifact name (metadata).",
			},
			"collection": map[string]any{
				"type":        "string",
				"description": "Optional collection ID/URN to assign the artifact to.",
			},
			"policy": map[string]any{
				"type":        "string",
				"description": "Optional access policy.",
			},
			"content": map[string]any{
				"type":        "array",
				"description": "Content parts (text or base64 sources).",
				"minItems":    1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type": map[string]any{
							"type":        "string",
							"description": "Part type (text, image, document, ...)",
						},
						"text": map[string]any{
							"type":        "string",
							"description": "Text content when type is text.",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Optional filename/path (used as tar path when multiple content parts).",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "Optional title for the content part (recorded in MANIFEST.json when multiple parts are packaged).",
						},
						"context": map[string]any{
							"type":        "string",
							"description": "Optional context about the content part (recorded in MANIFEST.json when multiple parts are packaged).",
						},
						"source": map[string]any{
							"type":        "object",
							"description": "Source for non-text content. Supports base64 or url.",
							"properties": map[string]any{
								"type":       map[string]any{"type": "string", "enum": []any{"base64", "url"}},
								"media_type": map[string]any{"type": "string", "description": "Mime type (for base64). For url sources, will be auto-detected from the HTTP response if not provided."},
								"data":       map[string]any{"type": "string", "description": "base64 data (when source.type=base64)"},
								"url":        map[string]any{"type": "string", "description": "URL to download (when source.type=url)"},
							},
						},
					},
					"required": []any{"type"},
				},
			},
		},
		"required": []any{"content"},
	}

	tool := mcp.NewToolWithRawSchema(
		"artifact_create",
		"Create an IVCAP artifact from LLM-style `content[]` parts. If multiple parts are provided they will be packaged into a .tar.gz before upload.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		b, err := a.JsonPayloadFromAny(args, logger)
		if err != nil {
			return nil, err
		}
		var parsed artifactCreateArgs
		if err := b.AsType(&parsed); err != nil {
			return nil, err
		}
		if len(parsed.Content) == 0 {
			return nil, fmt.Errorf("missing content")
		}

		adpt, err := createMCPAdapterFn(timeout)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Create payload (reader, mime, size) either from single part or tar.gz.
		payload, mime, size, err := mcpContentToArtifactPayload(ctxt, parsed.Content)
		if err != nil {
			return nil, err
		}

		creq := &sdk.CreateArtifactRequest{
			Name:       parsed.Name,
			Size:       size,
			Collection: parsed.Collection,
			Policy:     parsed.Policy,
		}
		resp, err := createArtifactFn(ctxt, creq, mime, size, nil, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}
		if resp == nil || resp.ID == nil || resp.DataHref == nil {
			return nil, fmt.Errorf("unexpected create artifact response")
		}
		artifactID := *resp.ID
		p, err := (*adpt).GetPath(*resp.DataHref)
		if err != nil {
			return nil, err
		}
		if err := uploadArtifactFn(ctxt, payload, size, 0, DEF_CHUNK_SIZE, p, adpt, true, logger); err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}

		art, err := readArtifactFn(ctxt, &sdk.ReadArtifactRequest{Id: artifactID}, adpt, logger)
		if err != nil {
			// Upload succeeded; still return minimal info.
			return mcp.NewToolResultJSON(map[string]any{"id": artifactID, "mime_type": mime, "size": size})
		}
		return mcp.NewToolResultJSON(map[string]any{
			"id":        artifactID,
			"name":      safeString(art.Name),
			"status":    safeString(art.Status),
			"mime_type": safeString(art.MimeType),
			"size":      art.Size,
		})
	}

	s.AddTool(tool, handler)
}

func mcpContentToArtifactPayload(ctx context.Context, parts []mcpContentPart) (io.Reader, string, int64, error) {
	if len(parts) == 1 {
		b, mime, err := mcpPartToBytes(ctx, parts[0])
		if err != nil {
			return nil, "", 0, err
		}
		return bytes.NewReader(b), mime, int64(len(b)), nil
	}
	// Multi-part: tar.gz
	b, err := tarGzFromParts(ctx, parts)
	if err != nil {
		return nil, "", 0, err
	}
	return bytes.NewReader(b), "application/gzip", int64(len(b)), nil
}

func mcpPartToBytes(ctx context.Context, p mcpContentPart) ([]byte, string, error) {
	switch p.Type {
	case "text":
		return []byte(p.Text), "text/plain; charset=utf-8", nil
	default:
		if p.Source == nil {
			return nil, "", fmt.Errorf("content part %q missing source", p.Type)
		}
		switch p.Source.Type {
		case "base64":
			decoded, err := base64.StdEncoding.DecodeString(p.Source.Data)
			if err != nil {
				return nil, "", fmt.Errorf("invalid base64 data: %w", err)
			}
			mt := p.Source.MediaType
			if mt == "" {
				mt = "application/octet-stream"
			}
			return decoded, mt, nil
		case "url":
			if p.Source.URL == "" {
				return nil, "", fmt.Errorf("source.type=url missing source.url")
			}
			b, mt, err := fetchURLBytesFn(ctx, p.Source.URL)
			if err != nil {
				return nil, "", err
			}
			// prefer provided media_type if set, else use fetched
			if p.Source.MediaType != "" {
				mt = p.Source.MediaType
			}
			if mt == "" {
				mt = "application/octet-stream"
			}
			return b, mt, nil
		default:
			return nil, "", fmt.Errorf("unsupported source.type %q", p.Source.Type)
		}
	}
}

func fetchURLBytes(ctx context.Context, u string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("url fetch failed: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	mt := resp.Header.Get("Content-Type")
	if mt != "" {
		// drop parameters to keep deterministic
		if m0, _, err := mime.ParseMediaType(mt); err == nil {
			mt = m0
		}
	}
	return b, mt, nil
}

type manifestEntry struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Title     string `json:"title,omitempty"`
	Context   string `json:"context,omitempty"`
	Size      int    `json:"size"`
}

func tarGzFromParts(ctx context.Context, parts []mcpContentPart) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	manifest := make([]manifestEntry, 0, len(parts))
	usedNames := map[string]bool{}

	for idx, p := range parts {
		data, detectedMT, err := mcpPartToBytes(ctx, p)
		if err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, err
		}
		name := p.Name
		if name == "" {
			name = defaultPartName(p, idx)
		}
		tarPath, err := sanitizeTarPath(name)
		if err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, err
		}
		if tarPath == "MANIFEST.json" {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, fmt.Errorf("content part name cannot be MANIFEST.json")
		}
		if usedNames[tarPath] {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, fmt.Errorf("duplicate tar path %q", tarPath)
		}
		usedNames[tarPath] = true
		hdr := &tar.Header{
			Name:    tarPath,
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, err
		}
		// update manifest (best-effort media type)
		mt := detectedMT
		manifest = append(manifest, manifestEntry{
			Path:      tarPath,
			Type:      p.Type,
			MediaType: mt,
			Title:     p.Title,
			Context:   p.Context,
			Size:      len(data),
		})
	}

	// Add manifest entry
	mb, err := json.MarshalIndent(map[string]any{"files": manifest}, "", "  ")
	if err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return nil, err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "MANIFEST.json", Mode: 0o644, Size: int64(len(mb)), ModTime: time.Now()}); err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return nil, err
	}
	if _, err := tw.Write(mb); err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return nil, err
	}

	if err := tw.Close(); err != nil {
		_ = gzw.Close()
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func defaultPartName(p mcpContentPart, idx int) string {
	// Prefer stable & readable.
	if p.Type == "text" {
		return fmt.Sprintf("part-%d.txt", idx+1)
	}
	// best-effort extension from media type
	ext := extFromMediaType("")
	if p.Source != nil {
		ext = extFromMediaType(p.Source.MediaType)
	}
	return fmt.Sprintf("part-%d.%s", idx+1, ext)
}

func extFromMediaType(mt string) string {
	mt = strings.ToLower(mt)
	if strings.Contains(mt, "png") {
		return "png"
	}
	if strings.Contains(mt, "jpeg") || strings.Contains(mt, "jpg") {
		return "jpg"
	}
	if strings.Contains(mt, "pdf") {
		return "pdf"
	}
	if strings.Contains(mt, "json") {
		return "json"
	}
	if strings.Contains(mt, "text") {
		return "txt"
	}
	return "bin"
}

func sanitizeTarPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty name")
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
