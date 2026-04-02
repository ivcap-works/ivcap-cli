package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

type artifactGetArgs struct {
	ID   string `json:"id"`
	Path string `json:"path,omitempty"`
}

// allow test stubbing
var downloadArtifactBytesFn = downloadArtifactBytes

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
		},
		"required": []any{"id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"artifact_get",
		"Fetch an IVCAP artifact. If `path` is provided and the artifact is a tar/tar.gz, return only that file (served from a small in-process cache for the last accessed tar artifact).",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, logger)
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

		adpt, err := createMCPAdapterFn(timeout)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		art, err := readArtifactFn(ctxt, &sdk.ReadArtifactRequest{Id: parsed.ID}, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}
		if art == nil || art.DataHref == nil {
			return nil, fmt.Errorf("artifact has no data")
		}
		mime := safeString(art.MimeType)
		dataURL := *art.DataHref

		// If caller asked for an internal tar path, attempt tar extraction.
		if parsed.Path != "" {
			b, _, err := getTarArtifactBytesCached(ctxt, parsed.ID, dataURL, mime, adpt)
			if err != nil {
				return nil, err
			}
			if !looksLikeTar(mime, b) {
				return nil, fmt.Errorf("artifact %q does not look like a tar/tar.gz, cannot use path", parsed.ID)
			}
			inner, innerMime, err := extractFromTarAuto(b, parsed.Path)
			if err != nil {
				return nil, err
			}
			return toolResultBlob(inner, innerMime)
		}

		// Otherwise, return entire artifact.
		data, err := downloadArtifactBytesFn(ctxt, dataURL, adpt)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}
		if mime == "" {
			mime = "application/octet-stream"
		}
		// Populate cache if it's a tar-ish type
		maybeUpdateTarCache(parsed.ID, data, mime)
		return toolResultBlob(data, mime)
	}

	s.AddTool(tool, handler)
}

func toolResultBlob(b []byte, mime string) (*mcp.CallToolResult, error) {
	if mime == "" {
		mime = "application/octet-stream"
	}
	enc := base64.StdEncoding.EncodeToString(b)
	// Use embedded resource so clients can render as a blob.
	res := mcp.BlobResourceContents{
		URI:      "urn:ivcap:artifact:data",
		MIMEType: mime,
		Blob:     enc,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewEmbeddedResource(res)},
		StructuredContent: map[string]any{
			"mime_type": mime,
			"size":      len(b),
		},
	}, nil
}

func downloadArtifactBytes(ctx context.Context, dataHref string, adpt *a.Adapter) ([]byte, error) {
	u, err := url.ParseRequestURI(dataHref)
	if err != nil {
		return nil, err
	}
	endpointPath := u.Path
	if u.RawQuery != "" {
		endpointPath = endpointPath + "?" + u.RawQuery
	}
	var out []byte
	handler := func(resp *http.Response, p string, logger *log.Logger) error {
		if resp.StatusCode >= 300 {
			return a.ProcessErrorResponse(resp, p, nil, logger)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		out = b
		return nil
	}
	if err := (*adpt).GetWithHandler(ctx, endpointPath, nil, handler, logger); err != nil {
		return nil, err
	}
	return out, nil
}

func getTarArtifactBytesCached(ctx context.Context, artifactID, dataHref, mime string, adpt *a.Adapter) ([]byte, string, error) {
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
	if mime == "" {
		mime = "application/octet-stream"
	}
	maybeUpdateTarCache(artifactID, data, mime)
	return data, mime, nil
}

func maybeUpdateTarCache(artifactID string, data []byte, mime string) {
	if !looksLikeTar(mime, data) {
		return
	}
	lastTarCache.mu.Lock()
	defer lastTarCache.mu.Unlock()
	lastTarCache.artifact = artifactID
	lastTarCache.raw = data
	lastTarCache.mediaType = mime
}

func looksLikeTar(mime string, data []byte) bool {
	mt := strings.ToLower(mime)
	if strings.Contains(mt, "tar") || strings.Contains(mt, "gzip") || strings.Contains(mt, "tgz") {
		return true
	}
	// Heuristic: gzip magic
	return len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b
}

func extractFromTarAuto(data []byte, innerPath string) ([]byte, string, error) {
	innerPath = strings.TrimPrefix(innerPath, "/")
	innerPath = path.Clean(innerPath)
	if innerPath == "." || strings.HasPrefix(innerPath, "../") {
		return nil, "", fmt.Errorf("invalid inner path: %q", innerPath)
	}

	// Try gzip first.
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, "", err
		}
		defer gzr.Close()
		return extractFromTarReader(tar.NewReader(gzr), innerPath)
	}
	return extractFromTarReader(tar.NewReader(bytes.NewReader(data)), innerPath)
}

func extractFromTarReader(tr *tar.Reader, innerPath string) ([]byte, string, error) {
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", err
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "/"))
		if name == innerPath {
			b, err := io.ReadAll(tr)
			if err != nil {
				return nil, "", err
			}
			return b, "application/octet-stream", nil
		}
	}
	return nil, "", fmt.Errorf("path %q not found in tar", innerPath)
}
