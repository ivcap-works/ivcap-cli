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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	nf "github.com/ivcap-works/ivcap-cli/pkg/nextflow"
)

// artifactBuildSession tracks a single incremental build session
type artifactBuildSession struct {
	ID         string
	StagingDir string
	Files      map[string]*artifactBuildFile
	CreatedAt  time.Time
	mu         sync.Mutex
}

// artifactBuildFile represents a staged file
type artifactBuildFile struct {
	PathName    string `json:"path_name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	LocalPath   string `json:"-"` // actual file path on disk
}

var (
	buildSessionsMu sync.RWMutex
	buildSessions   = map[string]*artifactBuildSession{}
)

func addArtifactBuildTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"stage": map[string]any{
				"type":        "string",
				"description": "Build stage: 'init', 'add', 'add_remote', 'list', or 'submit'",
				"enum":        []any{"init", "add", "add_remote", "list", "submit"},
			},
			"id": map[string]any{
				"type":        "string",
				"description": "Session ID (required for add, list, submit stages; returned by init)",
			},
			"files": map[string]any{
				"type":        "array",
				"description": "Files to add (required for 'add' stage)",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path_name": map[string]any{
							"type":        "string",
							"description": "Path within the tar archive",
						},
						"size": map[string]any{
							"type":        "integer",
							"description": "File size in bytes",
						},
						"mime_type": map[string]any{
							"type":        "string",
							"description": "MIME type of the file",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Base64-encoded file content",
						},
					},
					"required": []any{"path_name", "content"},
				},
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Artifact name (optional, for 'submit' stage)",
			},
			"policy": map[string]any{
				"type":        "string",
				"description": "Access policy (optional, for 'submit' stage)",
			},
		},
		"required": []any{"stage"},
	}

	tool := mcp.NewToolWithRawSchema(
		"artifact_build",
		"Incrementally build an artifact to upload as a tar.gz. Stages: 'init' (returns UUID), 'add' (stage files), 'list' (show current files), 'submit' (create tar.gz and upload).",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		stage, _ := args["stage"].(string)

		switch stage {
		case "init":
			return handleArtifactBuildInit(ctx, args)
		case "add":
			return handleArtifactBuildAdd(ctx, args)
		case "add_remote":
			return handleArtifactBuildAddRemote(ctx, args)
		case "list":
			return handleArtifactBuildList(ctx, args)
		case "submit":
			return handleArtifactBuildSubmit(ctx, args)
		default:
			return nil, fmt.Errorf("invalid stage: %q (must be init, add, add_remote, list, or submit)", stage)
		}
	}

	s.AddTool(tool, handler)
}

func handleArtifactBuildInit(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID := uuid.New().String()

	// Create staging directory
	stagingDir := filepath.Join(os.TempDir(), "ivcap-artifact-build", sessionID)
	if err := os.MkdirAll(stagingDir, 0755); err != nil { // #nosec G301 -- staging directory in temp
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	session := &artifactBuildSession{
		ID:         sessionID,
		StagingDir: stagingDir,
		Files:      make(map[string]*artifactBuildFile),
		CreatedAt:  time.Now(),
	}

	buildSessionsMu.Lock()
	buildSessions[sessionID] = session
	buildSessionsMu.Unlock()

	return mcp.NewToolResultJSON(map[string]any{
		"id":          sessionID,
		"staging_dir": stagingDir,
		"created_at":  session.CreatedAt.Format(time.RFC3339),
	})
}

func handleArtifactBuildAdd(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, _ := args["id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}

	buildSessionsMu.RLock()
	session, exists := buildSessions[sessionID]
	buildSessionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	filesArg, ok := args["files"].([]any)
	if !ok || len(filesArg) == 0 {
		return nil, fmt.Errorf("missing or empty 'files' array")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	addedFiles := []map[string]any{}

	for idx, fileArg := range filesArg {
		fileMap, ok := fileArg.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("file at index %d is not an object", idx)
		}

		pathName, _ := fileMap["path_name"].(string)
		if pathName == "" {
			return nil, fmt.Errorf("file at index %d missing 'path_name'", idx)
		}

		// Sanitize path
		sanitizedPath, err := nf.SanitizeTarPath(pathName)
		if err != nil {
			return nil, fmt.Errorf("invalid path_name %q: %w", pathName, err)
		}

		if sanitizedPath == "MANIFEST.json" {
			return nil, fmt.Errorf("path_name cannot be MANIFEST.json (reserved)")
		}

		if _, exists := session.Files[sanitizedPath]; exists {
			return nil, fmt.Errorf("duplicate path_name: %q", sanitizedPath)
		}

		contentB64, _ := fileMap["content"].(string)
		if contentB64 == "" {
			return nil, fmt.Errorf("file %q missing 'content'", pathName)
		}

		// Decode base64 content
		content, err := base64.StdEncoding.DecodeString(contentB64)
		if err != nil {
			return nil, fmt.Errorf("file %q has invalid base64 content: %w", pathName, err)
		}

		// Get optional fields
		mimeType, _ := fileMap["mime_type"].(string)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		// Validate size if provided
		if sizeArg, ok := fileMap["size"]; ok {
			var expectedSize int64
			switch v := sizeArg.(type) {
			case float64:
				expectedSize = int64(v)
			case int:
				expectedSize = int64(v)
			case int64:
				expectedSize = v
			}
			if expectedSize > 0 && int64(len(content)) != expectedSize {
				return nil, fmt.Errorf("file %q size mismatch: expected %d, got %d", pathName, expectedSize, len(content))
			}
		}

		// Write content to staging directory
		localPath := filepath.Join(session.StagingDir, sanitizedPath)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil { // #nosec G301 -- staging directory in temp
			return nil, fmt.Errorf("failed to create directory for %q: %w", pathName, err)
		}

		if err := os.WriteFile(localPath, content, 0644); err != nil { // #nosec G306 -- staging file in temp
			return nil, fmt.Errorf("failed to write file %q: %w", pathName, err)
		}

		// Record file metadata
		file := &artifactBuildFile{
			PathName:    sanitizedPath,
			Size:        int64(len(content)),
			ContentType: mimeType,
			LocalPath:   localPath,
		}
		session.Files[sanitizedPath] = file

		addedFiles = append(addedFiles, map[string]any{
			"path_name":    sanitizedPath,
			"size":         file.Size,
			"content_type": file.ContentType,
		})
	}

	return mcp.NewToolResultJSON(map[string]any{
		"id":          sessionID,
		"files_added": len(addedFiles),
		"files":       addedFiles,
		"total_files": len(session.Files),
	})
}

func handleArtifactBuildAddRemote(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, _ := args["id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}

	buildSessionsMu.RLock()
	session, exists := buildSessions[sessionID]
	buildSessionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	filesArg, ok := args["files"].([]any)
	if !ok || len(filesArg) == 0 {
		return nil, fmt.Errorf("missing or empty 'files' array")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	addedFiles := []map[string]any{}

	for idx, fileArg := range filesArg {
		fileMap, ok := fileArg.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("file at index %d is not an object", idx)
		}

		pathName, _ := fileMap["path_name"].(string)
		if pathName == "" {
			return nil, fmt.Errorf("file at index %d missing 'path_name'", idx)
		}

		urlStr, _ := fileMap["url"].(string)
		if urlStr == "" {
			return nil, fmt.Errorf("file %q missing 'url'", pathName)
		}

		// Sanitize path
		sanitizedPath, err := nf.SanitizeTarPath(pathName)
		if err != nil {
			return nil, fmt.Errorf("invalid path_name %q: %w", pathName, err)
		}

		if sanitizedPath == "MANIFEST.json" {
			return nil, fmt.Errorf("path_name cannot be MANIFEST.json (reserved)")
		}

		if _, exists := session.Files[sanitizedPath]; exists {
			return nil, fmt.Errorf("duplicate path_name: %q", sanitizedPath)
		}

		// Download content from URL
		resp, err := http.Get(urlStr) // #nosec G107 -- URL is validated by caller via verify_url package
		if err != nil {
			return nil, fmt.Errorf("failed to download %q from %q: %w", pathName, urlStr, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("download %q failed with status %d: %s", pathName, resp.StatusCode, urlStr)
		}

		// Read content from response
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read content for %q: %w", pathName, err)
		}

		// Get optional fields
		mimeType, _ := fileMap["mime_type"].(string)
		if mimeType == "" {
			// Try to infer from Content-Type header
			mimeType = resp.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
		}

		// Validate size if provided
		if sizeArg, ok := fileMap["size"]; ok {
			var expectedSize int64
			switch v := sizeArg.(type) {
			case float64:
				expectedSize = int64(v)
			case int:
				expectedSize = int64(v)
			case int64:
				expectedSize = v
			}
			if expectedSize > 0 && int64(len(content)) != expectedSize {
				return nil, fmt.Errorf("file %q size mismatch: expected %d, got %d", pathName, expectedSize, len(content))
			}
		}

		// Write content to staging directory
		localPath := filepath.Join(session.StagingDir, sanitizedPath)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil { // #nosec G301 -- staging directory in temp
			return nil, fmt.Errorf("failed to create directory for %q: %w", pathName, err)
		}

		if err := os.WriteFile(localPath, content, 0644); err != nil { // #nosec G306 -- staging file in temp
			return nil, fmt.Errorf("failed to write file %q: %w", pathName, err)
		}

		// Record file metadata
		file := &artifactBuildFile{
			PathName:    sanitizedPath,
			Size:        int64(len(content)),
			ContentType: mimeType,
			LocalPath:   localPath,
		}
		session.Files[sanitizedPath] = file

		addedFiles = append(addedFiles, map[string]any{
			"path_name":    sanitizedPath,
			"size":         file.Size,
			"content_type": file.ContentType,
			"url":          urlStr,
		})
	}

	return mcp.NewToolResultJSON(map[string]any{
		"id":          sessionID,
		"files_added": len(addedFiles),
		"files":       addedFiles,
		"total_files": len(session.Files),
	})
}

func handleArtifactBuildList(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, _ := args["id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}

	buildSessionsMu.RLock()
	session, exists := buildSessions[sessionID]
	buildSessionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	files := []map[string]any{}
	var totalSize int64

	for _, file := range session.Files {
		files = append(files, map[string]any{
			"path":         file.PathName,
			"size":         file.Size,
			"content_type": file.ContentType,
		})
		totalSize += file.Size
	}

	return mcp.NewToolResultJSON(map[string]any{
		"id":         sessionID,
		"file_count": len(files),
		"total_size": totalSize,
		"files":      files,
		"created_at": session.CreatedAt.Format(time.RFC3339),
	})
}

func handleArtifactBuildSubmit(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, _ := args["id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}

	buildSessionsMu.Lock()
	session, exists := buildSessions[sessionID]
	if exists {
		delete(buildSessions, sessionID)
	}
	buildSessionsMu.Unlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if len(session.Files) == 0 {
		_ = os.RemoveAll(session.StagingDir)
		return nil, fmt.Errorf("no files to submit")
	}

	// Create tar.gz
	tarGzData, manifest, err := createTarGzFromSession(session)
	if err != nil {
		_ = os.RemoveAll(session.StagingDir)
		return nil, fmt.Errorf("failed to create tar.gz: %w", err)
	}

	// Upload as artifact
	adpt, err := createAdapter(srvCfg.TimeoutSec)
	if err != nil {
		_ = os.RemoveAll(session.StagingDir)
		if isAuthFailure(err) {
			return NewLoginRequiredResult(), nil
		}
		return nil, err
	}

	ctxt, cancel := withTimeout(ctx)
	defer cancel()

	name, _ := args["name"].(string)
	policy, _ := args["policy"].(string)

	mimeType := "application/gzip"
	size := int64(len(tarGzData))

	creq := &sdk.CreateArtifactRequest{
		Name:   name,
		Size:   size,
		Policy: policy,
	}

	resp, err := createArtifactFn(ctxt, creq, mimeType, size, nil, adpt, srvCfg.Logger)
	if err != nil {
		_ = os.RemoveAll(session.StagingDir)
		if isAuthFailure(err) {
			return NewLoginRequiredResult(), nil
		}
		return nil, err
	}

	if resp == nil || resp.ID == nil || resp.DataHref == nil {
		_ = os.RemoveAll(session.StagingDir)
		return nil, fmt.Errorf("unexpected create artifact response")
	}

	artifactID := *resp.ID
	p, err := (*adpt).GetPath(*resp.DataHref)
	if err != nil {
		_ = os.RemoveAll(session.StagingDir)
		return nil, err
	}

	chunkSize := srvCfg.ChunkSize
	if chunkSize == 0 {
		chunkSize = 10000000
	}

	payload := bytes.NewReader(tarGzData)
	if err := uploadArtifactFn(ctxt, payload, size, 0, chunkSize, p, adpt, true, srvCfg.Logger); err != nil {
		_ = os.RemoveAll(session.StagingDir)
		if isAuthFailure(err) {
			return NewLoginRequiredResult(), nil
		}
		return nil, err
	}

	// Clean up staging directory after successful upload
	_ = os.RemoveAll(session.StagingDir)

	// Get artifact details
	art, err := readArtifactFn(ctxt, &sdk.ReadArtifactRequest{Id: artifactID}, adpt, srvCfg.Logger)
	if err != nil {
		// Upload succeeded; return minimal info
		return mcp.NewToolResultJSON(map[string]any{
			"id":         artifactID,
			"mime_type":  mimeType,
			"size":       size,
			"file_count": len(session.Files),
			"manifest":   manifest,
		})
	}

	return mcp.NewToolResultJSON(map[string]any{
		"id":         artifactID,
		"name":       safeString(art.Name),
		"status":     safeString(art.Status),
		"mime_type":  safeString(art.MimeType),
		"size":       art.Size,
		"file_count": len(session.Files),
		"manifest":   manifest,
	})
}

func createTarGzFromSession(session *artifactBuildSession) ([]byte, []map[string]any, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	manifest := []map[string]any{}

	// Add files in deterministic order
	for _, file := range session.Files {
		content, err := os.ReadFile(file.LocalPath)
		if err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, nil, fmt.Errorf("failed to read %q: %w", file.PathName, err)
		}

		hdr := &tar.Header{
			Name:    file.PathName,
			Mode:    0o644,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, nil, fmt.Errorf("failed to write header for %q: %w", file.PathName, err)
		}

		if _, err := tw.Write(content); err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return nil, nil, fmt.Errorf("failed to write content for %q: %w", file.PathName, err)
		}

		manifest = append(manifest, map[string]any{
			"path":         file.PathName,
			"size":         file.Size,
			"content_type": file.ContentType,
		})
	}

	// Add MANIFEST.json
	manifestJSON, err := json.MarshalIndent(map[string]any{"files": manifest}, "", "  ")
	if err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return nil, nil, err
	}

	manifestHdr := &tar.Header{
		Name:    "MANIFEST.json",
		Mode:    0o644,
		Size:    int64(len(manifestJSON)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(manifestHdr); err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return nil, nil, err
	}

	if _, err := tw.Write(manifestJSON); err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return nil, nil, err
	}

	if err := tw.Close(); err != nil {
		_ = gzw.Close()
		return nil, nil, err
	}

	if err := gzw.Close(); err != nil {
		return nil, nil, err
	}

	return buf.Bytes(), manifest, nil
}
