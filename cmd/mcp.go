// Copyright 2025-2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	mcppkg "github.com/ivcap-works/ivcap-cli/pkg/mcp"
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
		Short: "Start an MCP server for accessing all tools on an IVCAP platform",
		Long: `Start an MCP (Model Context Protocol) server.

In addition to MCP Tools for calling platform services, this server also exposes
the ivcap-cli release’s embedded agent skills via MCP Resources and Prompts
("MCP-Provisioned Skills").

Resources:
  - skills://manifest         JSON manifest of available skills
  - skills://catalog.json     JSON catalog (metadata + hashes; no bodies)
  - skills://CONTEXT.md       General agent best-practices for ivcap-cli
  - skills://SKILLS.md        Top-level skills tree index
  - skills://{name}/SKILL.md  Skill playbook body (markdown)
  - skills://file/{path}      Any embedded markdown file (e.g. category SKILLS.md)

Prompts:
  - use-ivcap-best-practices  Instructs an agent to load CONTEXT + relevant skills
`,
		GroupID: agentSupportGroupID,

		RunE: func(cmd *cobra.Command, args []string) error {
			s := mcppkg.NewServer(mcppkg.Config{
				Logger:        logger,
				Version:       rootCmd.Version,
				ToolSchema:    toolSchema,
				TimeoutSec:    timeout,
				ChunkSize:     DEF_CHUNK_SIZE,
				CreateAdapter: createMCPAdapter,
			})
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

func createMCPAdapter(timeoutSec int) (*a.Adapter, error) {
	ctxt, err := GetContextWithError("", true)
	if err != nil {
		return nil, err
	}

	accessToken := ""
	if accessTokenF != "" {
		accessToken = accessTokenF
	} else if envToken := os.Getenv(ACCESS_TOKEN_ENV); envToken != "" {
		accessToken = envToken
	} else if ctxt.AccessToken != "" && time.Now().Before(ctxt.AccessTokenExpiry) {
		// Only use cached context token if it hasn't expired.
		accessToken = ctxt.AccessToken
	}
	if accessToken == "" {
		return nil, mcppkg.ErrLoginRequired
	}

	url := ctxt.URL
	var headers *map[string]string
	if ctxt.Host != "" {
		headers = &(map[string]string{"Host": ctxt.Host})
	}
	adp, err := NewAdapter(url, accessToken, timeoutSec, headers)
	if err != nil {
		return nil, err
	}
	return adp, nil
}
