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
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

const mcpLoginRequiredMessage = "please run 'ivcap login' in your terminal to continue"

var errMCPLoginRequired = errors.New(mcpLoginRequiredMessage)

// allow test stubbing
var (
	listAspectFn          = sdk.ListAspect
	getAspectRawFn        = sdk.GetAspectRaw
	addUpdateAspectFn     = sdk.AddUpdateAspect
	listServicesRawFn     = sdk.ListServicesRaw
	createServiceJobRawFn = sdk.CreateServiceJobRaw
	createArtifactFn      = sdk.CreateArtifact
	uploadArtifactFn      = sdk.UploadArtifact
	readArtifactFn        = sdk.ReadArtifact
	createMCPAdapterFn    = createMCPAdapter
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
			s := newCLIMCPServer()
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

// We intentionally do not eagerly load tool aspects at startup.
// Tools are fetched on-demand via `select_tools`, using the platform's
// semantic service search.

// mcpDiscoveredTool caches tool definitions fetched from the platform.
// We keep these separate from the actual server registry so we can
// lazily expose tools to a session after a client calls select_tools().
type mcpDiscoveredTool struct {
	tool      mcp.Tool
	handler   server.ToolHandlerFunc
	serviceID string
}

type mcpDiscoveryState struct {
	mu sync.RWMutex

	// discovered holds all tools discovered from the platform, keyed by tool name.
	// This set can be larger than what is currently exposed to a client.
	discovered map[string]mcpDiscoveredTool
}

func newMCPDiscoveryState() *mcpDiscoveryState {
	return &mcpDiscoveryState{discovered: map[string]mcpDiscoveredTool{}}
}

func (d *mcpDiscoveryState) setDiscoveredTools(tools map[string]mcpDiscoveredTool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.discovered = tools
}

func newCLIMCPServer() *server.MCPServer {
	disco := newMCPDiscoveryState()
	s := server.NewMCPServer(
		"IVCAP MCP Server",
		"1.0.0",
		// We control list_changed explicitly.
		server.WithToolCapabilities(true),
		// Only expose the built-in discovery tool by default.
		// After select_tools, we update the allowlist for this session.
		server.WithToolFilter(filterToolsBySessionAllowlist),
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
	return s
}

// builtInToolNames are always visible (independent of select_tools allowlisting).
// Keep this list small and stable.
var builtInToolNames = map[string]bool{
	"select_tools":    true,
	"artifact_create": true,
	"artifact_get":    true,
	"aspect_search":   true,
	"aspect_get":      true,
	"aspect_create":   true,
	"nextflow_create": true,
	"nextflow_run":    true,
}

var (
	sessionToolAllowlistsMu sync.RWMutex
	// sessionID -> toolName -> allowed
	sessionToolAllowlists = map[string]map[string]bool{}
	// sessionID -> ordered list of selected tool names (sorted by relevance score)
	sessionToolOrder = map[string][]string{}
	// sessionID -> toolName -> relevance score
	sessionToolScores = map[string]map[string]float64{}
)

func setSessionAllowlist(sessionID string, orderedToolNames []string, toolScores map[string]float64) {
	sessionToolAllowlistsMu.Lock()
	defer sessionToolAllowlistsMu.Unlock()
	allowed := make(map[string]bool, len(orderedToolNames))
	for _, n := range orderedToolNames {
		allowed[n] = true
	}
	sessionToolAllowlists[sessionID] = allowed
	sessionToolOrder[sessionID] = orderedToolNames
	// toolScores is already per-tool; store as-is (read-only usage later)
	sessionToolScores[sessionID] = toolScores
}

func getSessionAllowlist(sessionID string) map[string]bool {
	sessionToolAllowlistsMu.RLock()
	defer sessionToolAllowlistsMu.RUnlock()
	if m, ok := sessionToolAllowlists[sessionID]; ok {
		// Return as-is (read-only usage).
		return m
	}
	return nil
}

func getSessionToolOrder(sessionID string) []string {
	sessionToolAllowlistsMu.RLock()
	defer sessionToolAllowlistsMu.RUnlock()
	return sessionToolOrder[sessionID]
}

// filterToolsBySessionAllowlist ensures that tools/list only returns:
//   - select_tools
//   - tools explicitly allowlisted for the current session via select_tools
//
// This does not rely on SessionWithTools (which is not implemented by stdio sessions).
func filterToolsBySessionAllowlist(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	// If there is no allowlist for the current session, behave as if only select_tools is allowed.
	sess := server.ClientSessionFromContext(ctx)
	if sess == nil {
		// Default: built-in tools only.
		res := make([]mcp.Tool, 0, 6)
		for _, t := range tools {
			if builtInToolNames[t.Name] {
				res = append(res, t)
			}
		}
		// Keep stable order: select_tools first, then the rest alphabetically.
		sort.Slice(res, func(i, j int) bool {
			if res[i].Name == "select_tools" {
				return true
			}
			if res[j].Name == "select_tools" {
				return false
			}
			return res[i].Name < res[j].Name
		})
		return res
	}
	sessionID := sess.SessionID()
	allowed := getSessionAllowlist(sessionID)
	order := getSessionToolOrder(sessionID)

	// Build lookup for stable output order.
	toolMap := make(map[string]mcp.Tool, len(tools))
	var selectTools *mcp.Tool
	for _, t := range tools {
		toolMap[t.Name] = t
		if t.Name == "select_tools" {
			tc := t
			selectTools = &tc
		}
	}

	res := make([]mcp.Tool, 0, 2+len(order))
	// Always include built-ins.
	if selectTools != nil {
		res = append(res, *selectTools)
	}
	// Keep stable order for built-ins after select_tools.
	builtInOrder := []string{"artifact_create", "artifact_get", "aspect_search", "aspect_get", "aspect_create", "nextflow_create", "nextflow_run"}
	for _, n := range builtInOrder {
		if t, ok := toolMap[n]; ok {
			res = append(res, t)
		}
	}
	if allowed == nil {
		return res
	}
	// Emit selected tools in score order.
	for _, name := range order {
		if !allowed[name] {
			continue
		}
		if t, ok := toolMap[name]; ok {
			res = append(res, t)
		}
	}
	return res
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"fn_schema"`
}

func parseTool(item map[string]any) (string, mcp.Tool, server.ToolHandlerFunc, string, error) {
	var ok bool
	var name, description, serviceID string
	var schema map[string]any

	if name, ok = item["name"].(string); !ok {
		return "", mcp.Tool{}, nil, "", fmt.Errorf("tool aspect missing 'name' field or not a string")
	}
	if description, ok = item["description"].(string); !ok {
		return "", mcp.Tool{}, nil, "", fmt.Errorf("tool aspect missing 'description' field or not a string")
	}
	if schema, ok = item["fn_schema"].(map[string]any); !ok {
		return "", mcp.Tool{}, nil, "", fmt.Errorf("tool aspect missing 'fn_schema' field or not an object")
	}
	if serviceID, ok = item["service-id"].(string); !ok {
		return "", mcp.Tool{}, nil, "", fmt.Errorf("tool aspect missing 'service-id' field or not a string")
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return run_tool(ctx, serviceID, request)
	}
	tool := mcp.NewToolWithRawSchema(
		name,
		description,
		MapToRaw(schema),
	)
	return name, tool, handler, serviceID, nil
}

// ---- Tool discovery / session exposure ----------------------------------------------------

func getDiscoveryState(s *server.MCPServer) *mcpDiscoveryState {
	if s == nil {
		return nil
	}
	// Use the server's internal context key-value store via instructions isn't available.
	// Instead we stash it in a package-level map keyed by pointer. This is safe because
	// `ivcap mcp` runs one server per process.
	// (We cannot modify mcp-go types.)
	discoveryStateByServerMu.Lock()
	defer discoveryStateByServerMu.Unlock()
	return discoveryStateByServer[s]
}

var discoveryStateByServer = map[*server.MCPServer]*mcpDiscoveryState{}
var discoveryStateByServerMu sync.Mutex

func setDiscoveryState(s *server.MCPServer, st *mcpDiscoveryState) {
	discoveryStateByServerMu.Lock()
	defer discoveryStateByServerMu.Unlock()
	discoveryStateByServer[s] = st
}

func addToolDiscoveryTool(s *server.MCPServer, disco *mcpDiscoveryState) {
	setDiscoveryState(s, disco)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"interest": map[string]any{
				"type":        "string",
				"description": "What you're trying to do; used to select and expose only relevant tools.",
			},
		},
		"required": []any{"interest"},
	}

	tool := mcp.NewToolWithRawSchema(
		"select_tools",
		"Discover and expose relevant tools for your current interest. After calling this, re-run tools/list to see the selected tools.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		interestVal, _ := args["interest"].(string)
		discoState := getDiscoveryState(server.ServerFromContext(ctx))
		if discoState == nil {
			return nil, fmt.Errorf("discovery state not available")
		}

		// Use platform semantic search to find relevant services, then resolve their tool aspects.
		matchingTools, toolScores, servicesFound, servicesWithoutTools, err := discoverToolsViaServiceSearch(ctx, interestVal, server.ServerFromContext(ctx))
		if err != nil {
			return nil, err
		}
		discoState.setDiscoveredTools(matchingTools)

		// At this point the platform already performed semantic filtering via service search,
		// so we expose all tools discovered from the matching services.
		matches := make([]mcpDiscoveredTool, 0, len(matchingTools))
		for _, mt := range matchingTools {
			matches = append(matches, mt)
		}

		// Update per-session allowlist.
		// NOTE: for stdio, mcp-go uses a static session id "stdio".
		// The SessionID may not always be available in every call context,
		// so fall back to "stdio" (safe for stdio mode).
		sess := server.ClientSessionFromContext(ctx)
		sessionID := "stdio"
		if sess != nil && sess.SessionID() != "" {
			sessionID = sess.SessionID()
		}
		// Sort tools by descending relevance score (from service search), then by name.
		sort.Slice(matches, func(i, j int) bool {
			si := toolScores[matches[i].tool.Name]
			sj := toolScores[matches[j].tool.Name]
			if si != sj {
				return si > sj
			}
			return matches[i].tool.Name < matches[j].tool.Name
		})

		ordered := make([]string, 0, len(matches))
		for _, mt := range matches {
			ordered = append(ordered, mt.tool.Name)
		}
		setSessionAllowlist(sessionID, ordered, toolScores)

		// Register discovered tools globally so they can be called via tools/call.
		// Important: do this *after* updating the allowlist so that the
		// tools/list_changed notification emitted by AddTools leads to a
		// tools/list response that already includes the newly selected tools.
		srv := server.ServerFromContext(ctx)
		if srv != nil {
			st := make([]server.ServerTool, 0, len(matches))
			for _, mt := range matches {
				st = append(st, server.ServerTool{Tool: mt.tool, Handler: mt.handler})
			}
			if len(st) > 0 {
				srv.AddTools(st...)
			}
		}

		// Notify the client that tools/list has changed.
		// Note: AddTools already emits list_changed to all initialized clients,
		// but we also send it to the current client explicitly to align with
		// the “select_tools triggers discovery” flow.
		if srv != nil {
			_ = srv.SendNotificationToClient(ctx, mcp.MethodNotificationToolsListChanged, nil)
		}

		// Return the list of selected tools (sorted by score) for convenience.
		type selectedTool struct {
			Name      string  `json:"name"`
			ServiceID string  `json:"service_id"`
			Score     float64 `json:"score"`
		}
		selectedTools := make([]selectedTool, 0, len(matches))
		selectedNames := make([]string, 0, len(matches))
		for _, mt := range matches {
			score := toolScores[mt.tool.Name]
			selectedTools = append(selectedTools, selectedTool{Name: mt.tool.Name, ServiceID: mt.serviceID, Score: score})
			selectedNames = append(selectedNames, mt.tool.Name)
		}
		return mcp.NewToolResultJSON(map[string]any{
			"selected":               selectedNames,
			"selected_tools":         selectedTools,
			"services_found":         servicesFound,
			"services_without_tools": servicesWithoutTools,
			"note":                   "Only services with tool-aspects (schema urn:sd-core:schema.ai-tool.1) can be exposed as MCP tools.",
		})
	}

	s.AddTool(tool, handler)
}

func discoverToolsViaServiceSearch(ctx context.Context, interest string, s *server.MCPServer) (map[string]mcpDiscoveredTool, map[string]float64, []string, []string, error) {
	if s == nil {
		return nil, nil, nil, nil, fmt.Errorf("internal error: missing server")
	}
	if interest == "" {
		return map[string]mcpDiscoveredTool{}, map[string]float64{}, nil, nil, nil
	}
	adpt, err := createMCPAdapterFn(timeout)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Ask platform for semantically relevant services.
	q := interest
	req := &sdk.ListRequest{Limit: 20, Search: &q}
	pyld, err := listServicesRawFn(ctx, req, adpt, logger)
	if err != nil {
		if isAuthFailure(err) {
			return nil, nil, nil, nil, errMCPLoginRequired
		}
		return nil, nil, nil, nil, err
	}
	serviceScores, serviceIDs, err := parseServiceListWithScores(pyld.AsBytes())
	if err != nil {
		return nil, nil, nil, nil, err
	}

	inv := map[string]mcpDiscoveredTool{}
	servicesFound := make([]string, 0, len(serviceIDs))
	servicesWithoutTools := make([]string, 0, len(serviceIDs))
	toolScores := map[string]float64{}
	if len(serviceIDs) == 0 {
		return inv, toolScores, servicesFound, servicesWithoutTools, nil
	}

	for _, serviceID := range serviceIDs {
		servicesFound = append(servicesFound, serviceID)

		// Fetch aspects describing tools for this service.
		selector := sdk.AspectSelector{
			Entity:         serviceID,
			SchemaPrefix:   toolSchema,
			IncludeContent: true,
			ListRequest: sdk.ListRequest{
				Limit: 50,
			},
		}
		aspects, _, err := listAspectFn(ctx, selector, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, nil, nil, nil, errMCPLoginRequired
			}
			return nil, nil, nil, nil, err
		}
		if aspects == nil || len(aspects.Items) == 0 {
			servicesWithoutTools = append(servicesWithoutTools, serviceID)
			continue
		}
		for _, aitem := range aspects.Items {
			c, ok := aitem.Content.(map[string]any)
			if !ok {
				continue
			}
			name, tool, handler, sid, err := parseTool(c)
			if err != nil {
				continue
			}
			// If the tool aspect doesn't carry service-id correctly, fall back to
			// the service we searched for.
			if sid == "" {
				sid = serviceID
			}
			inv[name] = mcpDiscoveredTool{tool: tool, handler: handler, serviceID: sid}
			toolScores[name] = serviceScores[serviceID]
		}
	}

	return inv, toolScores, servicesFound, servicesWithoutTools, nil
}

func parseServiceListWithScores(b []byte) (map[string]float64, []string, error) {
	// We parse raw JSON so we can capture the platform-provided `score` field.
	// The generated ServiceListResponseBody does not model it.
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, nil, err
	}
	scores := map[string]float64{}
	ids := make([]string, 0, len(raw.Items))
	for _, it := range raw.Items {
		id, _ := it["id"].(string)
		if id == "" {
			continue
		}
		score := 0.0
		switch v := it["score"].(type) {
		case float64:
			score = v
		case int:
			score = float64(v)
		case json.Number:
			if f, err := v.Float64(); err == nil {
				score = f
			}
		case string:
			if f, err := json.Number(v).Float64(); err == nil {
				score = f
			}
		}
		ids = append(ids, id)
		scores[id] = score
	}
	return scores, ids, nil
}

func run_tool(ctx context.Context, serviceID string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	logger.Info("Calling service", log.String("service-id", serviceID), log.Reflect("params", request.Params))
	args := request.Params.Arguments
	pyld, err := a.JsonPayloadFromAny(args, logger)
	if err != nil {
		return nil, err
	}
	adpt, err := createMCPAdapterFn(timeout)
	if err != nil {
		return nil, err
	}
	res, jobCreate, err := createServiceJobRawFn(ctx, serviceID, pyld, 0, adpt, logger)
	if err != nil {
		if isAuthFailure(err) {
			return nil, errMCPLoginRequired
		}
		return nil, err
	}
	if res.StatusCode() >= 300 {
		return nil, fmt.Errorf("service call failed: %d", res.StatusCode())
	}
	var result map[string]interface{}
	if jobCreate != nil {
		jobRes, err := waitForServiceJob(ctx, serviceID, jobCreate, adpt)
		if err != nil {
			return nil, err
		}
		result = jobRes
	} else {
		if result, err = res.AsObject(); err != nil {
			return nil, err
		}
	}
	return mcp.NewToolResultJSON(result)
}

func waitForServiceJob(
	ctx context.Context,
	serviceID string,
	jobCreate *sdk.JobCreateT,
	adpt *a.Adapter,
) (map[string]any, error) {
	jobID := jobCreate.JobID
	if jobCreate.ServiceID != "" {
		serviceID = jobCreate.ServiceID
	}

	// Mirror CLI behaviour: poll for completion (but keep it simple for MCP).
	maxChecks := 100
	wait := 2

	for tries := 0; tries < maxChecks; tries++ {
		time.Sleep(time.Duration(wait) * time.Second)
		job, pyld, err := sdk.ReadServiceJob(ctx, &sdk.ReadServiceJobRequest{ServiceId: serviceID, JobId: jobID}, adpt, logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, errMCPLoginRequired
			}
			return nil, err
		}
		status := "?"
		if job != nil && job.Status != nil {
			status = *job.Status
		}
		done := !(status == "?" || status == "scheduled" || status == "executing")
		if done {
			o, err := pyld.AsObject()
			if err != nil {
				return nil, err
			}
			// Job responses wrap the actual result payload in 'result-content'.
			if rc, ok := o["result-content"].(map[string]any); ok {
				return rc, nil
			}
			return nil, fmt.Errorf("unexpected result content from job")
		}
	}
	return nil, fmt.Errorf("timed out waiting for job to finish")
}

func isAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	var unauth *a.UnauthorizedError
	if errors.As(err, &unauth) {
		return true
	}
	var apiErr *a.ApiError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 401 || apiErr.StatusCode == 403
	}
	// Also treat missing/expired token (empty) as auth failure when surfaced explicitly.
	return errors.Is(err, errMCPLoginRequired)
}

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
		return nil, errMCPLoginRequired
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

func MapToRaw(m map[string]any) json.RawMessage {
	b, err := json.Marshal(m) // or json.MarshalIndent(m, "", "  ")
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot convert map into json: %v", err))
	}
	return json.RawMessage(b)
}
