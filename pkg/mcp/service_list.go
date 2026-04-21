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

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tool: service_list
//
// Lightweight listing of services (avoid schema bloat). This directly wraps the
// core service list endpoint.

type serviceListArgs struct {
	Limit  int     `json:"limit,omitempty"`
	Page   *string `json:"page,omitempty"`
	Search *string `json:"search,omitempty"`
}

func addServiceListTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "integer", "description": "Max number of services to return."},
			"page":   map[string]any{"type": "string", "description": "Optional paging token."},
			"search": map[string]any{"type": "string", "description": "Optional search string."},
		},
	}

	tool := mcp.NewToolWithRawSchema(
		"service_list",
		"List available services (lightweight; returns the platform response without tool-aspect expansion).",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed serviceListArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		limit := parsed.Limit
		if limit == 0 {
			limit = 20
		}
		req0 := &sdk.ListRequest{Limit: limit, Page: parsed.Page, Search: parsed.Search}
		res, err := listServicesRawFn(ctxt, req0, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
			}
			return nil, err
		}
		if res == nil {
			return nil, fmt.Errorf("unexpected empty response")
		}

		if o, err := res.AsObject(); err == nil {
			return mcp.NewToolResultJSON(o)
		}
		return mcp.NewToolResultText(string(res.AsBytes())), nil
	}

	s.AddTool(tool, handler)
}
