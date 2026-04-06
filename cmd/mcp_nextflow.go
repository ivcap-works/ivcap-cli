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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	yaml "gopkg.in/yaml.v2"
)

// Built-in MCP tools for Nextflow service creation + job run.

// allow test stubbing
var extractFileFromTarBytesFn = extractFileFromTarBytes

type nextflowSource struct {
	// Path inside the assembled archive.
	Path string `json:"path"`
	// One of: text, base64, url, artifact
	Type string `json:"type"`
	// For text
	Text string `json:"text,omitempty"`
	// For base64
	Base64 string `json:"base64,omitempty"`
	// For url
	URL string `json:"url,omitempty"`
	// For artifact
	ArtifactID   string `json:"artifact_id,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	// Optional mime type for manifest.
	MediaType string `json:"media_type,omitempty"`
}

type nextflowCreateArgs struct {
	// Service ID/URN to create/update service description for.
	ServiceID string `json:"service_id"`
	// Optional name override for created pipeline artifact.
	Name string `json:"name,omitempty"`
	// Optional collection id/urn
	Collection string `json:"collection,omitempty"`
	// Optional policy
	Policy string `json:"policy,omitempty"`
	// Sources to assemble into a single tar.gz pipeline package.
	Sources []nextflowSource `json:"sources"`
}

type nextflowRunArgs struct {
	ServiceID string `json:"service_id"`
	// Inline job input payload (JSON object), if provided.
	Input map[string]any `json:"input,omitempty"`
	// URN of aspect containing job parameters (alternative to inline input)
	AspectURN string `json:"aspect_urn,omitempty"`
	// If true, wait for job completion and return result-content.
	Watch bool `json:"watch,omitempty"`
}

func addNextflowCreateTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service_id": map[string]any{
				"type":        "string",
				"description": "Service URN/ID for the Nextflow service definition to create/update.",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Optional name for the pipeline artifact (metadata).",
			},
			"collection": map[string]any{
				"type":        "string",
				"description": "Optional collection ID/URN to assign the pipeline artifact to.",
			},
			"policy": map[string]any{
				"type":        "string",
				"description": "Optional access policy for the pipeline artifact.",
			},
			"sources": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "Files to assemble into the Nextflow package tar.gz. Each entry becomes a file at `path` inside the archive.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":          map[string]any{"type": "string", "description": "Path in the assembled tar.gz (e.g. main.nf, nextflow.config, bin/script.sh, ivcap-tool.yaml)."},
						"type":          map[string]any{"type": "string", "enum": []any{"text", "base64", "url", "artifact"}},
						"text":          map[string]any{"type": "string"},
						"base64":        map[string]any{"type": "string"},
						"url":           map[string]any{"type": "string"},
						"artifact_id":   map[string]any{"type": "string", "description": "Artifact URN/ID (when type=artifact)."},
						"artifact_path": map[string]any{"type": "string", "description": "Optional path inside a tar artifact (when type=artifact)."},
						"media_type":    map[string]any{"type": "string", "description": "Optional mime-type hint (stored in MANIFEST.json)."},
					},
					"required": []any{"path", "type"},
				},
			},
		},
		"required": []any{"service_id", "sources"},
	}

	tool := mcp.NewToolWithRawSchema(
		"nextflow_create",
		"Assemble a Nextflow pipeline package (.tar.gz) from a list of sources, upload as an artifact, validate `ivcap-tool.yaml`, and publish/update the service description aspect for the given service.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		b, err := a.JsonPayloadFromAny(args, logger)
		if err != nil {
			return nil, err
		}
		var parsed nextflowCreateArgs
		if err := b.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.ServiceID == "" {
			return nil, fmt.Errorf("missing service_id")
		}
		if len(parsed.Sources) == 0 {
			return nil, fmt.Errorf("missing sources")
		}

		adpt, err := createMCPAdapterFn(timeout)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Assemble package tar.gz.
		pkgBytes, manifest, err := tarGzFromNextflowSources(ctxt, parsed.Sources, adpt)
		if err != nil {
			return nil, err
		}

		// Validate that archive contains a tool description. Prefer ivcap.yaml if present,
		// falling back to ivcap-tool.yaml.
		toolHdr, foundPath, err := loadNextflowToolHeaderFromArchiveBytes(pkgBytes)
		if err != nil {
			return nil, err
		}
		if toolHdr == nil {
			return nil, fmt.Errorf("neither %q nor %q found in assembled archive", nextflowSimpleToolFileName, nextflowToolFileName)
		}

		artifactName := parsed.Name
		if artifactName == "" {
			artifactName = toolHdr.Name
			if artifactName == "" {
				artifactName = "nextflow-pipeline"
			}
		}

		// Create + upload artifact.
		mime := "application/gzip"
		size := int64(len(pkgBytes))
		creq := &sdk.CreateArtifactRequest{
			Name:       artifactName,
			Size:       size,
			Collection: parsed.Collection,
			Policy:     parsed.Policy,
			Meta:       map[string]string{"ivcap.nextflow.manifest": manifest},
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
		if err := uploadArtifactFn(ctxt, bytes.NewReader(pkgBytes), size, 0, DEF_CHUNK_SIZE, p, adpt, true, logger); err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}

		// Publish service description aspect (same logic as `ivcap nextflow create`).
		svc := buildNextflowServiceDescription(toolHdr, parsed.ServiceID, artifactID)
		aspectID, err := upsertServiceDescriptionAspect(ctxt, parsed.ServiceID, svc)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}

		return mcp.NewToolResultJSON(map[string]any{
			"ok":                       true,
			"service_id":               parsed.ServiceID,
			"pipeline_artifact_urn":    artifactID,
			"service_aspect_record_id": aspectID,
			"tool": map[string]any{
				"name":        toolHdr.Name,
				"description": toolHdr.Description,
				"service_id":  toolHdr.ServiceID,
				"source":      foundPath,
			},
		})
	}

	s.AddTool(tool, handler)
}

func addNextflowRunTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service_id": map[string]any{"type": "string", "description": "Service URN/ID to run."},
			"input":      map[string]any{"type": "object", "description": "Inline job input payload (JSON object)."},
			"aspect_urn": map[string]any{"type": "string", "description": "Alternative to input: URN of an aspect containing job parameters."},
			"watch":      map[string]any{"type": "boolean", "description": "If true, wait for completion and return the job result-content."},
		},
		"required": []any{"service_id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"nextflow_run",
		"Run (create a job for) a Nextflow service. Provide either inline `input` or a `aspect_urn` referencing request parameters.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		b, err := a.JsonPayloadFromAny(args, logger)
		if err != nil {
			return nil, err
		}
		var parsed nextflowRunArgs
		if err := b.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.ServiceID == "" {
			return nil, fmt.Errorf("missing service_id")
		}
		if parsed.Input == nil && parsed.AspectURN == "" {
			return nil, fmt.Errorf("missing input or aspect_urn")
		}
		if parsed.Input != nil && parsed.AspectURN != "" {
			return nil, fmt.Errorf("provide only one of input or aspect_urn")
		}

		adpt, err := createMCPAdapterFn(timeout)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		var pyld a.Payload
		if parsed.Input != nil {
			// Use raw args conversion to preserve types.
			jp, err := a.JsonPayloadFromAny(parsed.Input, logger)
			if err != nil {
				return nil, err
			}
			pyld = jp
		} else {
			j := fmt.Sprintf(CREATE_FROM_ASPECT, parsed.AspectURN, parsed.ServiceID)
			if pyld, err = a.LoadPayloadFromBytes([]byte(j), false); err != nil {
				return nil, err
			}
		}

		res, jobCreate, err := createServiceJobRawFn(ctxt, parsed.ServiceID, pyld, 0, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}
		if res != nil && res.StatusCode() >= 300 {
			return nil, fmt.Errorf("service call failed: %d", res.StatusCode())
		}

		// Async job mode.
		if jobCreate != nil {
			if parsed.Watch {
				out, err := waitForServiceJob(ctxt, parsed.ServiceID, jobCreate, adpt)
				if err != nil {
					return nil, err
				}
				// Attach job-id too.
				return mcp.NewToolResultJSON(map[string]any{
					"job_id": jobCreate.JobID,
					"result": out,
				})
			}
			return mcp.NewToolResultJSON(map[string]any{"job_id": jobCreate.JobID})
		}

		// Immediate response mode.
		reply, err := res.AsObject()
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultJSON(reply)
	}

	s.AddTool(tool, handler)
}

// --- Pipeline tar assembly helpers -------------------------------------------------

type nextflowManifestEntry struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Source    string `json:"source,omitempty"`
	Size      int    `json:"size"`
}

func tarGzFromNextflowSources(ctx context.Context, sources []nextflowSource, adpt *a.Adapter) ([]byte, string, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	defer func() {
		// best effort; ignore close errors here (returned earlier)
		_ = tw.Close()
		_ = gzw.Close()
	}()

	used := map[string]bool{}
	manifest := make([]nextflowManifestEntry, 0, len(sources))

	for i, src := range sources {
		if strings.TrimSpace(src.Path) == "" {
			return nil, "", fmt.Errorf("sources[%d].path is required", i)
		}
		tarPath, err := sanitizeTarPath(src.Path)
		if err != nil {
			return nil, "", fmt.Errorf("invalid sources[%d].path: %w", i, err)
		}
		if tarPath == "MANIFEST.json" {
			return nil, "", fmt.Errorf("sources[%d].path cannot be MANIFEST.json", i)
		}
		if used[tarPath] {
			return nil, "", fmt.Errorf("duplicate path %q", tarPath)
		}
		used[tarPath] = true

		data, mt, sourceDesc, err := nextflowSourceToBytes(ctx, src, adpt)
		if err != nil {
			return nil, "", err
		}
		if mt == "" {
			mt = src.MediaType
		}

		hdr := &tar.Header{Name: tarPath, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, "", err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, "", err
		}

		manifest = append(manifest, nextflowManifestEntry{Path: tarPath, Type: src.Type, MediaType: mt, Source: sourceDesc, Size: len(data)})
	}

	mb, err := json.MarshalIndent(map[string]any{"files": manifest}, "", "  ")
	if err != nil {
		return nil, "", err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "MANIFEST.json", Mode: 0o644, Size: int64(len(mb)), ModTime: time.Now()}); err != nil {
		return nil, "", err
	}
	if _, err := tw.Write(mb); err != nil {
		return nil, "", err
	}

	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gzw.Close(); err != nil {
		return nil, "", err
	}
	// manifest is returned also as a compact JSON string, so callers can store it in artifact meta.
	compact, _ := json.Marshal(map[string]any{"files": manifest})
	return buf.Bytes(), string(compact), nil
}

func nextflowSourceToBytes(ctx context.Context, src nextflowSource, adpt *a.Adapter) ([]byte, string, string, error) {
	switch src.Type {
	case "text":
		return []byte(src.Text), "text/plain; charset=utf-8", "text", nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(src.Base64)
		if err != nil {
			return nil, "", "", fmt.Errorf("invalid base64 for %q: %w", src.Path, err)
		}
		mt := src.MediaType
		if mt == "" {
			mt = "application/octet-stream"
		}
		return decoded, mt, "base64", nil
	case "url":
		if src.URL == "" {
			return nil, "", "", fmt.Errorf("url source for %q missing url", src.Path)
		}
		b, mt, err := fetchURLBytesFn(ctx, src.URL)
		if err != nil {
			return nil, "", "", err
		}
		if src.MediaType != "" {
			mt = src.MediaType
		}
		if mt == "" {
			mt = "application/octet-stream"
		}
		return b, mt, "url:" + src.URL, nil
	case "artifact":
		if src.ArtifactID == "" {
			return nil, "", "", fmt.Errorf("artifact source for %q missing artifact_id", src.Path)
		}
		// Use same logic as artifact_get to optionally extract a file from a tar artifact.
		art, err := readArtifactFn(ctx, &sdk.ReadArtifactRequest{Id: src.ArtifactID}, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, "", "", errMCPLoginRequired
			}
			return nil, "", "", err
		}
		if art == nil || art.DataHref == nil {
			return nil, "", "", fmt.Errorf("artifact %q has no data", src.ArtifactID)
		}
		mime := safeString(art.MimeType)
		dataURL := *art.DataHref
		data, err := downloadArtifactBytesFn(ctx, dataURL, adpt)
		if err != nil {
			if isAuthFailure(err) {
				return nil, "", "", errMCPLoginRequired
			}
			return nil, "", "", err
		}
		if src.ArtifactPath != "" {
			inner, innerMT, err := extractFromTarAuto(data, src.ArtifactPath)
			if err != nil {
				return nil, "", "", err
			}
			if src.MediaType != "" {
				innerMT = src.MediaType
			}
			return inner, innerMT, fmt.Sprintf("artifact:%s#%s", src.ArtifactID, path.Clean(strings.TrimPrefix(src.ArtifactPath, "/"))), nil
		}
		if src.MediaType != "" {
			mime = src.MediaType
		}
		if mime == "" {
			mime = "application/octet-stream"
		}
		return data, mime, "artifact:" + src.ArtifactID, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported source type %q for %q", src.Type, src.Path)
	}
}

func extractFileFromTarBytes(archive []byte, fileName string) ([]byte, string, error) {
	fileName = path.Clean(strings.TrimPrefix(fileName, "/"))
	if fileName == "." || fileName == "" {
		return nil, "", fmt.Errorf("invalid target name: %q", fileName)
	}

	// Try gzip first (magic bytes).
	if len(archive) > 2 && archive[0] == 0x1f && archive[1] == 0x8b {
		gzr, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, "", err
		}
		defer gzr.Close()
		return extractFileFromTarReader(tar.NewReader(gzr), fileName)
	}
	return extractFileFromTarReader(tar.NewReader(bytes.NewReader(archive)), fileName)
}

func loadNextflowToolHeaderFromArchiveBytes(archive []byte) (*nextflowToolHeader, string, error) {
	// Prefer simplified ivcap.yaml.
	if b, foundPath, err := extractFileFromTarBytesFn(archive, nextflowSimpleToolFileName); err != nil {
		return nil, "", err
	} else if b != nil {
		var simple nextflowSimpleToolHeader
		dec := yaml.NewDecoder(bytes.NewReader(b))
		if err := dec.Decode(&simple); err != nil {
			return nil, "", fmt.Errorf("while parsing %s extracted from %s: %w", foundPath, "assembled-archive", err)
		}
		tool, err := convertSimpleToolToToolHeader(&simple)
		if err != nil {
			return nil, "", fmt.Errorf("invalid %s extracted from %s: %w", foundPath, "assembled-archive", err)
		}
		return tool, foundPath, nil
	}

	// Fallback to ivcap-tool.yaml.
	if toolYAML, foundPath, err := extractFileFromTarBytesFn(archive, nextflowToolFileName); err != nil {
		return nil, "", err
	} else if toolYAML != nil {
		var toolHdr nextflowToolHeader
		dec := yaml.NewDecoder(bytes.NewReader(toolYAML))
		if err := dec.Decode(&toolHdr); err != nil {
			return nil, "", fmt.Errorf("while parsing %s extracted from %s: %w", foundPath, "assembled-archive", err)
		}
		if err := validateFnSchema(toolHdr.FnSchema); err != nil {
			return nil, "", fmt.Errorf("invalid fn-schema in %s: %w", foundPath, err)
		}
		return &toolHdr, foundPath, nil
	}

	return nil, "", nil
}
