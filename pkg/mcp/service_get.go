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

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
)

// MCP tool: service_get
//
// Fetches the service tool-aspect(s) (schema urn:sd-core:schema.ai-tool.1 by default)
// for a service id/URN.
//
// IMPORTANT: This intentionally does NOT return the underlying Datafabric aspect
// record wrapper (entity, schema, record id, etc). It returns ONLY the aspect
// content objects.

type serviceGetArgs struct {
	ID string `json:"id"`
}

func addServiceGetTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Service URN/ID.",
			},
		},
		"required": []any{"id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"service_get",
		"Fetch a service tool-aspect description/schema by service ID/URN (returns only aspect content; hides Datafabric record fields).",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed serviceGetArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.ID == "" {
			return nil, fmt.Errorf("missing id")
		}

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		toolSchema := srvCfg.ToolSchema
		if toolSchema == "" {
			toolSchema = "urn:sd-core:schema.ai-tool.1"
		}

		selector := sdk.AspectSelector{
			Entity:         parsed.ID,
			SchemaPrefix:   toolSchema,
			IncludeContent: true,
			ListRequest: sdk.ListRequest{
				Limit: 50,
			},
		}
		aspects, _, err := listAspectFn(ctxt, selector, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
			}
			return nil, err
		}
		// Return only the first aspect content (no datafabric wrapper).
		// In most deployments there is exactly one tool-aspect per service.
		toolCount := 0
		var toolContent map[string]any
		if aspects != nil {
			for _, it := range aspects.Items {
				c, ok := it.Content.(map[string]any)
				if !ok || c == nil {
					continue
				}
				toolCount++
				if toolContent == nil {
					toolContent = c
				}
			}
		}

		// Return only the tool-aspect content object itself (no wrapper).
		// If no tool-aspect is found, return an empty object (consistent with other
		// MCP tools returning JSON).
		if toolContent == nil {
			return mcp.NewToolResultJSON(map[string]any{})
		}
		// If multiple tools are present, we still only return the first one.
		_ = toolCount
		_ = toolSchema
		_ = parsed
		return mcp.NewToolResultJSON(toolContent)
	}

	s.AddTool(tool, handler)
}
