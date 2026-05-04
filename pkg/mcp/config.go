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
	"errors"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	log "go.uber.org/zap"

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// LoginRequiredMessage is returned when an MCP tool invocation requires auth
// but no usable access token is available.
const LoginRequiredMessage = "please run 'ivcap context login' in your terminal to continue"

// ErrLoginRequired is a sentinel error used to signal that authentication is required.
var ErrLoginRequired = errors.New(LoginRequiredMessage)

// NewLoginRequiredResult creates a CallToolResult with isError=true for authentication failures.
// This returns a proper MCP tool result instead of a JSON-RPC error, providing a better
// user experience for AI agents.
func NewLoginRequiredResult() *mcpgo.CallToolResult {
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.NewTextContent("Authentication required. Your IVCAP session has expired. Please run 'ivcap context login' in your terminal to refresh your credentials, then retry."),
		},
		IsError: true,
	}
}

// Config provides CLI-owned dependencies and settings to the MCP server.
//
// Note: the MCP server itself lives in pkg/ so it does not depend on cmd/.
// Any CLI-specific concerns (config files, flags, env var resolution) should be
// handled by cmd/ and injected here.
type Config struct {
	Logger *log.Logger

	// Version is reported to MCP clients as the server's version (serverInfo.version).
	// The CLI should set this to match the ivcap-cli build version.
	Version string

	// ToolSchema is the schema URN prefix used when discovering tool aspects.
	ToolSchema string

	// TimeoutSec is used both for adapter construction and request timeouts.
	TimeoutSec int

	// ChunkSize is used by artifact uploads.
	ChunkSize int64

	// CreateAdapter must return an authenticated adapter.
	CreateAdapter func(timeoutSec int) (*a.Adapter, error)

	// WithLogging enables JSON-RPC request/response logging to a file.
	WithLogging bool

	// LogDir specifies the directory for MCP log files (defaults to /tmp).
	LogDir string
}

// NewServer constructs an MCP server exposing IVCAP tools.
func NewServer(cfg Config) *mcpserver.MCPServer {
	srvCfg = cfg

	disco := newMCPDiscoveryState()
	ver := cfg.Version
	if ver == "" {
		ver = "dev"
	}

	s := mcpserver.NewMCPServer(
		"IVCAP MCP Server",
		ver,
		// We control list_changed explicitly.
		mcpserver.WithToolCapabilities(true),
		// Enable resources + prompts so clients (e.g. Claude) can discover and load
		// embedded skills via resources/list + resources/read, and see the setup prompt.
		mcpserver.WithResourceCapabilities(false, false),
		mcpserver.WithPromptCapabilities(false),
		// Only expose the built-in discovery tool by default.
		// After select_tools, we update the allowlist for this session.
		mcpserver.WithToolFilter(filterToolsBySessionAllowlist),
	)

	// Always expose a single built-in tool for discovery.
	addToolDiscoveryTool(s, disco)
	// Always expose built-in tools that are implemented locally (not discovered from platform services).
	addArtifactCreateTool(s)
	addArtifactGetTool(s)
	addAspectSearchTool(s)
	addAspectGetTool(s)
	addAspectCreateTool(s)
	addArtifactBuildTool(s)
	addServiceListTool(s)
	addServiceGetTool(s)
	addServiceRunTool(s)
	addJobStatusTool(s)
	addNextflowCreateTool(s)
	addNextflowRunTool(s)
	addSkillsResourcesAndPrompts(s)
	addSkillsBridgeTools(s) // Workaround for clients that don't expose resources/read to LLMs

	// Ensure we surface a stable built-in list_changed method constant, even if unused.
	_ = mcpgo.MethodNotificationToolsListChanged
	return s
}

// srvCfg is set by NewServer and used by tool handlers.
//
// This is process-scoped state; `ivcap mcp` runs one server per process.
// Tests may overwrite this.
var srvCfg Config
