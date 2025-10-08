// Copyright 2025 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVarP(&toolSchema, "tool-schema", "s", "urn:sd-core:schema.ai-tool.1", "the schema URN used for describing MCP tools")
	mcpCmd.Flags().IntVar(&mcpPort, "port", -1, "optional port to open for SSE connection to MCP server")
}

var (
	toolSchema string
	mcpPort    int

	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Start an MCP server for all tools on an IVCAP platform",

		RunE: func(cmd *cobra.Command, args []string) error {
			s := server.NewMCPServer("IVCAP MCP Server", "1.0.0")
			if err := addTools(s); err != nil {
				cobra.CheckErr(fmt.Sprintf("Cannot add tools: %v", err))
			}
			if mcpPort > 0 {
				logger.Info("MCP Proxy Server starting as SSE server...", log.Int("port", mcpPort))
				hs := server.NewSSEServer(s,
					server.WithSSEEndpoint("/mcp"),
				)
				if err := hs.Start(fmt.Sprintf("localhost:%d", mcpPort)); err != nil {
					cobra.CheckErr(fmt.Sprintf("Server error: %v", err))
				}
			} else {
				logger.Info("MCP Proxy Server starting in STDIO mode...")
				if err := server.ServeStdio(s); err != nil {
					cobra.CheckErr(fmt.Sprintf("Server error: %v", err))
				}
			}
			return nil
		},
	}
)

func addTools(s *server.MCPServer) error {
	selector := sdk.AspectSelector{
		SchemaPrefix:   toolSchema,
		IncludeContent: true,
		ListRequest: sdk.ListRequest{
			Limit: 50,
		},
	}
	ctxt := context.Background()
	if list, _, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
		for _, item := range list.Items {
			if c, ok := item.Content.(map[string]any); ok {
				if err2 := addTool(c, s); err2 != nil {
					// only log errors here, so that one bad tool doesn't stop all the others from being added
					logger.Warn("Cannot add tool", log.String("id", *item.ID), log.Error(err2))
				}
			} else {
				return fmt.Errorf("unexpected content type for aspect '%s'", *item.ID)
			}
		}
		return nil
	} else {
		return err
	}
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"fn_schema"`
}

func addTool(item map[string]any, s *server.MCPServer) error {
	var ok bool
	var name, description, serviceID string
	var schema map[string]any

	if name, ok = item["name"].(string); !ok {
		return fmt.Errorf("tool aspect missing 'name' field or not a string")
	}
	if description, ok = item["description"].(string); !ok {
		return fmt.Errorf("tool aspect missing 'description' field or not a string")
	}
	if schema, ok = item["fn_schema"].(map[string]any); !ok {
		return fmt.Errorf("tool aspect missing 'fn_schema' field or not an object")
	}
	if serviceID, ok = item["service-id"].(string); !ok {
		return fmt.Errorf("tool aspect missing 'service-id' field or not a string")
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return run_tool(ctx, serviceID, request)
	}
	tool := mcp.NewToolWithRawSchema(
		name,
		description,
		MapToRaw(schema), // The Input Schema defined above
	)
	logger.Debug("Registering tool", log.String("name", name), log.String("service-id", serviceID))
	s.AddTool(tool, handler)
	return nil
}

func run_tool(ctx context.Context, serviceID string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	logger.Info("Calling service", log.String("service-id", serviceID), log.Reflect("params", request.Params))
	args := request.Params.Arguments
	pyld, err := a.JsonPayloadFromAny(args, logger)
	if err != nil {
		return nil, err
	}
	res, jobCreate, err := sdk.CreateServiceJobRaw(ctx, serviceID, pyld, 0, CreateAdapter(true), logger)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() >= 300 {
		return nil, fmt.Errorf("service call failed: %d", res.StatusCode())
	}
	var result map[string]interface{}
	if jobCreate != nil {
		_, res, err = watchJob(ctx, jobCreate.JobID, 100, 2)
		if err != nil {
			return nil, err
		}
		if o, err := res.AsObject(); err != nil {
			return nil, err
		} else {
			var ok bool
			if result, ok = o["result-content"].(map[string]any); !ok {
				return nil, fmt.Errorf("unexpected result content from job")
			}
		}
	} else {
		if result, err = res.AsObject(); err != nil {
			return nil, err
		}
	}
	return mcp.NewToolResultJSON(result)
}

func MapToRaw(m map[string]any) json.RawMessage {
	b, err := json.Marshal(m) // or json.MarshalIndent(m, "", "  ")
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot convert map into json: %v", err))
	}
	return json.RawMessage(b)
}
