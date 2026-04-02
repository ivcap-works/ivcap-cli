package cmd

import (
	"context"
	"fmt"
	"time"

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
		pyld, err := a.JsonPayloadFromAny(args, logger)
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

		adpt, err := createMCPAdapterFn(timeout)
		if err != nil {
			return nil, err
		}
		ctxt, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		res, err := getAspectRawFn(ctxt, parsed.ID, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
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
