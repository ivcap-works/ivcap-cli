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
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	nf "github.com/ivcap-works/ivcap-cli/pkg/nextflow"
)

// Built-in MCP tools for Nextflow service creation + job run.

type nextflowCreateArgs struct {
	// Service ID/URN to create/update service description for.
	ServiceID string `json:"service_id"`
	// Artifact URN/ID containing the pipeline tar.gz package.
	ArtifactID string `json:"artifact_id"`
	// Optional name for display purposes (metadata only).
	Name string `json:"name,omitempty"`
	// Optional policy
	Policy string `json:"policy,omitempty"`
}

type nextflowRunArgs struct {
	ServiceID string `json:"service_id"`
	// Inline job input payload (JSON object), if provided.
	Input map[string]any `json:"input,omitempty"`
	// URN of aspect containing job parameters (alternative to inline input)
	AspectURN string `json:"aspect_urn,omitempty"`
}

func addNextflowCreateTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service_id": map[string]any{
				"type":        "string",
				"description": "Service URN/ID for the Nextflow service definition to create/update. MUST be in format 'urn:ivcap:service:<uuid>' where <uuid> is a valid UUIDv5 that you generate. The caller is responsible for creating this service ID. (Note: This requirement may change in future versions to auto-generate service IDs.)",
			},
			"artifact_id": map[string]any{
				"type":        "string",
				"description": "Artifact URN/ID containing the pipeline tar.gz package. Use the artifact_build tool to create and upload the artifact first.",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Optional name for display purposes (metadata only).",
			},
			"policy": map[string]any{
				"type":        "string",
				"description": "Optional access policy.",
			},
		},
		"required": []any{"service_id", "artifact_id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"nextflow_create",
		"Deploy a Nextflow pipeline from a pre-built artifact. The artifact must contain the pipeline tar.gz package with ivcap.yaml or ivcap-tool.yaml. Use the artifact_build tool to create and upload the artifact first. For pipeline development best practices: see skills://nextflow-mcp-tools/SKILL.md (MCP tools usage), skills://nextflow-pipeline-deployment/SKILL.md (deployment), or skills://nextflow-mcp-debugging/SKILL.md (debugging). Use resources/read (preferred) or the built-in read_skill tool.",
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
		if parsed.ArtifactID == "" {
			return nil, fmt.Errorf("missing artifact_id")
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

		// Fetch artifact metadata to get the data URL
		art, err := readArtifactFn(ctxt, &sdk.ReadArtifactRequest{Id: parsed.ArtifactID}, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, fmt.Errorf("failed to read artifact %s: %w", parsed.ArtifactID, err)
		}
		if art == nil || art.DataHref == nil {
			return nil, fmt.Errorf("artifact %s has no data", parsed.ArtifactID)
		}

		// Download and validate the artifact
		pkgBytes, err := downloadArtifactBytesFn(ctxt, *art.DataHref, adpt)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, fmt.Errorf("failed to download artifact %s: %w", parsed.ArtifactID, err)
		}

		// Validate that archive contains a tool description. Prefer ivcap.yaml if present,
		// falling back to ivcap-tool.yaml.
		toolHdr, foundPath, err := nf.LoadToolHeaderFromArchiveBytes(pkgBytes)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		if toolHdr == nil {
			return nil, fmt.Errorf("neither %q nor %q found in artifact %s", nf.SimpleToolFileName, nf.ToolFileName, parsed.ArtifactID)
		}

		// Publish service description aspect (same logic as `ivcap nextflow create`).
		svc := nf.BuildServiceDescription(toolHdr, parsed.ServiceID, parsed.ArtifactID)
		aspectID, err := nf.UpsertServiceDescriptionAspect(ctxt, parsed.ServiceID, svc, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}

		return mcp.NewToolResultJSON(map[string]any{
			"ok":                       true,
			"service_id":               parsed.ServiceID,
			"pipeline_artifact_urn":    parsed.ArtifactID,
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
		},
		"required": []any{"service_id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"nextflow_run",
		"Run (create a job for) a Nextflow service. Provide either inline `input` or a `aspect_urn` referencing request parameters. Returns either: (1) Fast path: immediate result if job completes within 30s, or (2) Slow path: job metadata with job_id and polling instructions if still running. Use job_status tool to check long-running jobs. For details on handling long-running jobs, read MCP resource: skills://ivcap-service-long-running/SKILL.md (use resources/read).",
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
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		var pyld a.Payload
		if parsed.Input != nil {
			// Use raw args conversion to preserve types.
			jp, err := a.JsonPayloadFromAny(parsed.Input, srvCfg.Logger)
			if err != nil {
				if isAuthFailure(err) {
					return NewLoginRequiredResult(), nil
				}
				return nil, err
			}
			pyld = jp
		} else {
			j := fmt.Sprintf(sdk.CreateFromAspectTemplate, parsed.AspectURN, parsed.ServiceID)
			if pyld, err = a.LoadPayloadFromBytes([]byte(j), false); err != nil {
				return nil, err
			}
		}

		// ╔══════════════════════════════════════════════════════════════════════════╗
		// ║ TEMPORARY WORKAROUND - REMOVE WHEN IVCAP API NO LONGER REQUIRES $schema ║
		// ╚══════════════════════════════════════════════════════════════════════════╝
		// Ensure the payload has a $schema field (required by IVCAP API for now)
		pyld, err = a.EnsureSchemaField(pyld)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure $schema field: %w", err)
		}

		res, jobCreate, err := createServiceJobRawFn(ctxt, parsed.ServiceID, pyld, 0, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		if res != nil && res.StatusCode() >= 300 {
			return nil, fmt.Errorf("service call failed: %d", res.StatusCode())
		}

		// Async job mode - always use optimistic wait with 30-second timeout
		if jobCreate != nil {
			waitResult, err := waitForServiceJobOptimistic(ctxt, parsed.ServiceID, jobCreate, adpt, 30)
			if err != nil {
				return nil, err
			}

			if waitResult.IsComplete {
				// Fast path - job completed
				return mcp.NewToolResultJSON(map[string]any{
					"job_id": waitResult.JobID,
					"status": waitResult.Status,
					"result": waitResult.Result,
				})
			}

			// Slow path - job still running
			pollAfter := estimatePollInterval(waitResult.Status)
			return mcp.NewToolResultJSON(map[string]any{
				"job_id":  waitResult.JobID,
				"status":  waitResult.Status,
				"message": waitResult.Message,
				"_meta": map[string]any{
					"job_id":             waitResult.JobID,
					"status":             waitResult.Status,
					"poll_after_seconds": pollAfter,
				},
			})
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
