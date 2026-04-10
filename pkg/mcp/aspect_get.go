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

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tool: aspect_get

type aspectGetArgs struct {
	ID string `json:"id"`
}

func addAspectGetTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Aspect URN/ID.",
			},
		},
		"required": []any{"id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"aspect_get",
		"Get an aspect record by id/URN.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed aspectGetArgs
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

		res, err := getAspectRawFn(ctxt, parsed.ID, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
			}
			return nil, err
		}
		if o, err := res.AsObject(); err == nil {
			return mcp.NewToolResultJSON(o)
		}
		// fallback to bytes -> json
		return mcp.NewToolResultText(string(res.AsBytes())), nil
	}

	s.AddTool(tool, handler)
}
