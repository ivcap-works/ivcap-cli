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
	"bytes"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	nf "github.com/ivcap-works/ivcap-cli/pkg/nextflow"
)

// Built-in MCP tools for Nextflow service creation + job run.

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

// Map MCP structs to pkg/nextflow structs.
func toPkgSources(in []nextflowSource) []nf.Source {
	out := make([]nf.Source, 0, len(in))
	for _, s := range in {
		out = append(out, nf.Source{
			Path:         s.Path,
			Type:         s.Type,
			Text:         s.Text,
			Base64:       s.Base64,
			URL:          s.URL,
			ArtifactID:   s.ArtifactID,
			ArtifactPath: s.ArtifactPath,
			MediaType:    s.MediaType,
		})
	}
	return out
}

func tarGzFromSourcesForMCP(ctx context.Context, sources []nextflowSource, adpt *a.Adapter) ([]byte, string, error) {
	return nf.TarGzFromSources(ctx, toPkgSources(sources), adpt, srvCfg.Logger, fetchURLBytesFn, downloadArtifactBytesFn)
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
		b, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
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

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		// Assemble package tar.gz.
		pkgBytes, manifest, err := tarGzFromSourcesForMCP(ctxt, parsed.Sources, adpt)
		if err != nil {
			return nil, err
		}

		// Validate that archive contains a tool description. Prefer ivcap.yaml if present,
		// falling back to ivcap-tool.yaml.
		toolHdr, foundPath, err := nf.LoadToolHeaderFromArchiveBytes(pkgBytes)
		if err != nil {
			return nil, err
		}
		if toolHdr == nil {
			return nil, fmt.Errorf("neither %q nor %q found in assembled archive", nf.SimpleToolFileName, nf.ToolFileName)
		}

		artifactName := parsed.Name
		if artifactName == "" {
			artifactName = toolHdr.Name
			if artifactName == "" {
				artifactName = "nextflow-pipeline"
			}
		}

		// Create + upload artifact.
		mimeType := "application/gzip"
		size := int64(len(pkgBytes))
		creq := &sdk.CreateArtifactRequest{
			Name:       artifactName,
			Size:       size,
			Collection: parsed.Collection,
			Policy:     parsed.Policy,
			Meta:       map[string]string{"ivcap.nextflow.manifest": manifest},
		}
		resp, err := createArtifactFn(ctxt, creq, mimeType, size, nil, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
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
		chunkSize := srvCfg.ChunkSize
		if chunkSize == 0 {
			chunkSize = 10000000
		}
		if err := uploadArtifactFn(ctxt, bytes.NewReader(pkgBytes), size, 0, chunkSize, p, adpt, true, srvCfg.Logger); err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
			}
			return nil, err
		}

		// Publish service description aspect (same logic as `ivcap nextflow create`).
		svc := nf.BuildServiceDescription(toolHdr, parsed.ServiceID, artifactID)
		aspectID, err := nf.UpsertServiceDescriptionAspect(ctxt, parsed.ServiceID, svc, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
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
		b, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
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

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		var pyld a.Payload
		if parsed.Input != nil {
			// Use raw args conversion to preserve types.
			jp, err := a.JsonPayloadFromAny(parsed.Input, srvCfg.Logger)
			if err != nil {
				return nil, err
			}
			pyld = jp
		} else {
			j := fmt.Sprintf(sdk.CreateFromAspectTemplate, parsed.AspectURN, parsed.ServiceID)
			if pyld, err = a.LoadPayloadFromBytes([]byte(j), false); err != nil {
				return nil, err
			}
		}

		res, jobCreate, err := createServiceJobRawFn(ctxt, parsed.ServiceID, pyld, 0, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
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
