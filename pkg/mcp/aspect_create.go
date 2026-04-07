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
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tool: aspect_create

type aspectCreateArgs struct {
	Entity string         `json:"entity"`
	Schema string         `json:"schema"`
	Policy string         `json:"policy,omitempty"`
	Body   map[string]any `json:"body"`
}

func addAspectCreateTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entity": map[string]any{
				"type":        "string",
				"description": "Entity URN/ID to attach the aspect to.",
			},
			"schema": map[string]any{
				"type":        "string",
				"description": "Aspect schema URN.",
			},
			"policy": map[string]any{
				"type":        "string",
				"description": "Optional access policy.",
			},
			"body": map[string]any{
				"type":                 "object",
				"description":          "Aspect content. If it does not include a $schema field, the server will inject it from the `schema` parameter.",
				"additionalProperties": true,
			},
		},
		"required": []any{"entity", "schema", "body"},
	}

	tool := mcp.NewToolWithRawSchema(
		"aspect_create",
		"Create (add) a new aspect record for an entity + schema.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed aspectCreateArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.Entity == "" {
			return nil, fmt.Errorf("missing entity")
		}
		if parsed.Schema == "" {
			return nil, fmt.Errorf("missing schema")
		}
		if parsed.Body == nil {
			return nil, fmt.Errorf("missing body")
		}

		// Ensure $schema is present, matching CLI behavior.
		if _, ok := parsed.Body["$schema"]; !ok {
			parsed.Body["$schema"] = parsed.Schema
		}
		b, err := json.Marshal(parsed.Body)
		if err != nil {
			return nil, err
		}

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		res, err := addUpdateAspectFn(ctxt, true, parsed.Entity, parsed.Schema, parsed.Policy, b, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
			}
			return nil, err
		}
		if o, err := res.AsObject(); err == nil {
			return mcp.NewToolResultJSON(o)
		}
		return mcp.NewToolResultText(string(res.AsBytes())), nil
	}

	s.AddTool(tool, handler)
}
