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
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCP tool: verify_url
//
// Verify the existence of a URL by performing an HTTP HEAD request.
// Returns status code and headers information.

type verifyURLArgs struct {
	URL string `json:"url"`
}

func addVerifyURLTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to verify. Should be a complete URL (e.g., https://example.com/path)",
			},
		},
		"required": []any{"url"},
	}

	tool := mcp.NewToolWithRawSchema(
		"verify_url",
		"Verify the existence of a URL by performing an HTTP HEAD request. Returns status code, content type, and content length. Useful for agents to check if a URL is accessible before attempting to download data.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		urlVal, ok := args["url"].(string)
		if !ok || urlVal == "" {
			return nil, fmt.Errorf("missing or invalid 'url' parameter")
		}

		// Perform HEAD request with timeout
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, urlVal, nil)
		if err != nil {
			return mcp.NewToolResultJSON(map[string]any{
				"url":     urlVal,
				"success": false,
				"error":   fmt.Sprintf("Invalid URL: %v", err),
			})
		}

		resp, err := client.Do(headReq)
		if err != nil {
			return mcp.NewToolResultJSON(map[string]any{
				"url":     urlVal,
				"success": false,
				"error":   fmt.Sprintf("Request failed: %v", err),
			})
		}
		defer func() { _ = resp.Body.Close() }()

		// Extract relevant headers
		result := map[string]any{
			"url":            urlVal,
			"success":        resp.StatusCode < 400,
			"status_code":    resp.StatusCode,
			"status":         resp.Status,
			"content_type":   resp.Header.Get("Content-Type"),
			"content_length": resp.Header.Get("Content-Length"),
			"last_modified":  resp.Header.Get("Last-Modified"),
			"cache_control":  resp.Header.Get("Cache-Control"),
			"etag":           resp.Header.Get("ETag"),
		}

		// Add additional debug info for non-2xx/3xx responses
		if resp.StatusCode >= 400 {
			result["error"] = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		return mcp.NewToolResultJSON(result)
	}

	s.AddTool(tool, handler)
}
