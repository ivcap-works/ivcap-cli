// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO)
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

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tool: service_run
//
// Invoke any service by service_id + input payload. This is a generic escape hatch
// (independent of tool-aspect discovery).

type serviceRunArgs struct {
	ServiceID string         `json:"service_id"`
	Input     map[string]any `json:"input,omitempty"`
}

func addServiceRunTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service_id": map[string]any{"type": "string", "description": "Service URN/ID to run."},
			"input":      map[string]any{"type": "object", "description": "Inline job input payload (JSON object)."},
		},
		"required": []any{"service_id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"service_run",
		"Invoke any service by service_id and input payload. Returns either: (1) Fast path: immediate result if job completes within 30s, or (2) Slow path: job metadata with job_id and polling instructions if still running. Use job_status tool to check long-running jobs. See skills://ivcap-service-long-running/SKILL.md for details.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		b, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed serviceRunArgs
		if err := b.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.ServiceID == "" {
			return nil, fmt.Errorf("missing service_id")
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
			// Some services accept empty payloads; send an empty object.
			pyld, err = a.LoadPayloadFromBytes([]byte(`{}`), false)
			if err != nil {
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
