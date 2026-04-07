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
const LoginRequiredMessage = "please run 'ivcap login' in your terminal to continue"

// ErrLoginRequired is a sentinel error used to signal that authentication is required.
var ErrLoginRequired = errors.New(LoginRequiredMessage)

// Config provides CLI-owned dependencies and settings to the MCP server.
//
// Note: the MCP server itself lives in pkg/ so it does not depend on cmd/.
// Any CLI-specific concerns (config files, flags, env var resolution) should be
// handled by cmd/ and injected here.
type Config struct {
	Logger *log.Logger

	// ToolSchema is the schema URN prefix used when discovering tool aspects.
	ToolSchema string

	// TimeoutSec is used both for adapter construction and request timeouts.
	TimeoutSec int

	// ChunkSize is used by artifact uploads.
	ChunkSize int64

	// CreateAdapter must return an authenticated adapter.
	CreateAdapter func(timeoutSec int) (*a.Adapter, error)
}

// NewServer constructs an MCP server exposing IVCAP tools.
func NewServer(cfg Config) *mcpserver.MCPServer {
	srvCfg = cfg

	disco := newMCPDiscoveryState()
	s := mcpserver.NewMCPServer(
		"IVCAP MCP Server",
		"1.0.0",
		// We control list_changed explicitly.
		mcpserver.WithToolCapabilities(true),
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
	addNextflowCreateTool(s)
	addNextflowRunTool(s)
	addSkillsResourcesAndPrompts(s)

	// Ensure we surface a stable built-in list_changed method constant, even if unused.
	_ = mcpgo.MethodNotificationToolsListChanged
	return s
}

// srvCfg is set by NewServer and used by tool handlers.
//
// This is process-scoped state; `ivcap mcp` runs one server per process.
// Tests may overwrite this.
var srvCfg Config
