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
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tool: aspect_search
// A thin wrapper around the DataFabric aspect listing endpoint.

type aspectSearchArgs struct {
	Entity         string `json:"entity,omitempty"`
	SchemaPrefix   string `json:"schema_prefix,omitempty"`
	IncludeContent bool   `json:"include_content,omitempty"`
	// JSON path filter on aspect content; see CLI: datafabric query --content-path
	ContentPath string `json:"content_path,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Page        string `json:"page,omitempty"`
}

func addAspectSearchTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entity": map[string]any{
				"type":        "string",
				"description": "Entity URN/ID to filter by.",
			},
			"schema_prefix": map[string]any{
				"type":        "string",
				"description": "Schema URN/prefix to filter by.",
			},
			"include_content": map[string]any{
				"type":        "boolean",
				"description": "If true, include aspect content in the list response.",
				"default":     false,
			},
			"content_path": map[string]any{
				"type":        "string",
				"description": "Optional JSON-path filter on aspect content (passed through as 'aspect-path').",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max number of items to return.",
				"minimum":     1,
			},
			"page": map[string]any{
				"type":        "string",
				"description": "Opaque paging cursor (as returned by previous list responses).",
			},
		},
	}

	tool := mcp.NewToolWithRawSchema(
		"aspect_search",
		"Search/list aspects in the DataFabric by entity/schema (optionally including aspect content).",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, logger)
		if err != nil {
			return nil, err
		}
		var parsed aspectSearchArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.Entity == "" && parsed.SchemaPrefix == "" && parsed.Page == "" {
			return nil, fmt.Errorf("need at least one of entity, schema_prefix or page")
		}

		adpt, err := createMCPAdapterFn(timeout)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		sel := sdk.AspectSelector{
			Entity:         parsed.Entity,
			SchemaPrefix:   parsed.SchemaPrefix,
			IncludeContent: parsed.IncludeContent,
			ListRequest: sdk.ListRequest{
				Limit: parsed.Limit,
			},
		}
		if parsed.Page != "" {
			sel.ListRequest.Page = &parsed.Page
		}
		if parsed.ContentPath != "" {
			sel.JsonFilter = &parsed.ContentPath
		}

		list, raw, err := listAspectFn(ctxt, sel, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}
		// Prefer returning the raw payload for forward-compatibility.
		if raw != nil {
			if o, err := raw.AsObject(); err == nil {
				return mcp.NewToolResultJSON(o)
			}
			// Fall back to typed list.
		}
		return mcp.NewToolResultJSON(list)
	}

	s.AddTool(tool, handler)
}
