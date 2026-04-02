package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	aspect "github.com/ivcap-works/ivcap-core-api/http/aspect"
)

type fakePayload struct{ b []byte }

func (p *fakePayload) AsType(r interface{}) error                { return json.Unmarshal(p.b, r) }
func (p *fakePayload) AsObject() (map[string]interface{}, error) { return nil, nil }
func (p *fakePayload) AsArray() ([]interface{}, error)           { return nil, nil }
func (p *fakePayload) AsBytes() []byte                           { return p.b }
func (p *fakePayload) AsReader() (io.Reader, int64)              { return bytes.NewReader(p.b), int64(len(p.b)) }
func (p *fakePayload) IsEmpty() bool                             { return len(p.b) == 0 }
func (p *fakePayload) Header(key string) string                  { return "" }
func (p *fakePayload) ContentType() string                       { return "application/json" }
func (p *fakePayload) StatusCode() int                           { return 200 }

func TestMCPToolsList_Unauthorised_ReturnsLoginRequiredMessage(t *testing.T) {
	old := listAspectFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		listAspectFn = old
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) {
		// returning a nil adapter is ok because listAspectFn won't use it
		return nil, nil
	}
	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *log.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		return nil, nil, &a.UnauthorizedError{}
	}

	s := newCLIMCPServer()
	msg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	out := s.HandleMessage(context.Background(), msg)

	errResp, ok := out.(mcp.JSONRPCError)
	if !ok {
		t.Fatalf("expected JSONRPCError, got %T", out)
	}
	if errResp.Error.Message != mcpLoginRequiredMessage {
		t.Fatalf("expected message %q, got %q", mcpLoginRequiredMessage, errResp.Error.Message)
	}
}

func TestMCPToolsList_InitiallyOnlyHasSelectTools(t *testing.T) {
	old := listAspectFn
	oldListServices := listServicesRawFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		listAspectFn = old
		listServicesRawFn = oldListServices
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) {
		return nil, nil
	}
	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *log.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		// No tools returned from platform. That's fine.
		return &aspect.ListResponseBody{}, nil, nil
	}
	listServicesRawFn = func(ctxt context.Context, cmd *sdk.ListRequest, adpt *a.Adapter, logger *log.Logger) (a.Payload, error) {
		return nil, nil
	}

	s := newCLIMCPServer()

	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)

	// initialize session (so it can accept notifications later)
	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	out := s.HandleMessage(ctx, msg)

	res, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}

	b, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var parsed struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}
	if len(parsed.Tools) != 6 {
		t.Fatalf("expected 6 tools initially, got %d", len(parsed.Tools))
	}
	if parsed.Tools[0].Name != "select_tools" {
		t.Fatalf("expected first tool to be select_tools, got %q", parsed.Tools[0].Name)
	}
	got := map[string]bool{}
	for _, t := range parsed.Tools {
		got[t.Name] = true
	}
	if !got["select_tools"] || !got["artifact_create"] || !got["artifact_get"] || !got["aspect_search"] || !got["aspect_get"] || !got["aspect_create"] {
		t.Fatalf("expected built-in tools select_tools, artifact_create, artifact_get, aspect_search, aspect_get, aspect_create; got %+v", got)
	}
}

func TestMCPToolsList_SelectToolsExpandsList(t *testing.T) {
	oldListServices := listServicesRawFn
	oldListAspect := listAspectFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		listServicesRawFn = oldListServices
		listAspectFn = oldListAspect
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }

	// service search returns a single service
	listServicesRawFn = func(ctxt context.Context, cmd *sdk.ListRequest, adpt *a.Adapter, logger *log.Logger) (a.Payload, error) {
		b := []byte(`{"items":[{"id":"urn:ivcap:service:test"}]}`)
		return &fakePayload{b: b}, nil
	}

	// tool aspects for that service
	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *log.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		resp := &aspect.ListResponseBody{}
		resp.Items = []*aspect.AspectListItemRTResponseBody{
			{Content: map[string]any{
				"name":        "test_tool",
				"description": "A test tool",
				"fn_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
				"service-id": "urn:ivcap:service:test",
			}},
		}
		return resp, nil, nil
	}

	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)

	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	// Call select_tools
	selMsg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"select_tools","arguments":{"interest":"test"}}}`)
	_ = s.HandleMessage(ctx, selMsg)

	// List again, should now contain built-ins + test_tool (via allowlist + tool filter)
	listMsg := json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	out := s.HandleMessage(ctx, listMsg)
	res, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var parsed struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}
	if len(parsed.Tools) != 7 {
		t.Fatalf("expected 7 tools after select_tools, got %d", len(parsed.Tools))
	}
	got := map[string]bool{}
	for _, t := range parsed.Tools {
		got[t.Name] = true
	}
	if !got["select_tools"] || !got["artifact_create"] || !got["artifact_get"] || !got["aspect_search"] || !got["aspect_get"] || !got["aspect_create"] || !got["test_tool"] {
		t.Fatalf("expected built-ins plus test_tool, got %+v", got)
	}
}

func TestMCPToolsList_SelectToolsReportsServicesWithoutTools(t *testing.T) {
	oldListServices := listServicesRawFn
	oldListAspect := listAspectFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		listServicesRawFn = oldListServices
		listAspectFn = oldListAspect
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }

	listServicesRawFn = func(ctxt context.Context, cmd *sdk.ListRequest, adpt *a.Adapter, logger *log.Logger) (a.Payload, error) {
		b := []byte(`{"items":[{"id":"urn:ivcap:service:with-tool"},{"id":"urn:ivcap:service:no-tool"}]}`)
		return &fakePayload{b: b}, nil
	}

	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *log.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		resp := &aspect.ListResponseBody{}
		switch selector.Entity {
		case "urn:ivcap:service:with-tool":
			resp.Items = []*aspect.AspectListItemRTResponseBody{
				{Content: map[string]any{
					"name":        "tool_a",
					"description": "A tool",
					"fn_schema": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
					"service-id": "urn:ivcap:service:with-tool",
				}},
			}
		default:
			resp.Items = []*aspect.AspectListItemRTResponseBody{}
		}
		return resp, nil, nil
	}

	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)

	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	selMsg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"select_tools","arguments":{"interest":"x"}}}`)
	out := s.HandleMessage(ctx, selMsg)
	res, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var parsed struct {
		StructuredContent struct {
			Selected             []string `json:"selected"`
			ServicesFound        []string `json:"services_found"`
			ServicesWithoutTools []string `json:"services_without_tools"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}
	if len(parsed.StructuredContent.Selected) != 1 || parsed.StructuredContent.Selected[0] != "tool_a" {
		t.Fatalf("unexpected selected tools: %+v", parsed.StructuredContent.Selected)
	}
	if len(parsed.StructuredContent.ServicesFound) != 2 {
		t.Fatalf("expected 2 services_found, got %+v", parsed.StructuredContent.ServicesFound)
	}
	if len(parsed.StructuredContent.ServicesWithoutTools) != 1 || parsed.StructuredContent.ServicesWithoutTools[0] != "urn:ivcap:service:no-tool" {
		t.Fatalf("unexpected services_without_tools: %+v", parsed.StructuredContent.ServicesWithoutTools)
	}
}

func TestMCPToolsList_SelectToolsSortsByScore(t *testing.T) {
	oldListServices := listServicesRawFn
	oldListAspect := listAspectFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		listServicesRawFn = oldListServices
		listAspectFn = oldListAspect
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }

	// Return 2 services with different scores. The tool from the higher-score
	// service should appear first in tools/list after select_tools.
	listServicesRawFn = func(ctxt context.Context, cmd *sdk.ListRequest, adpt *a.Adapter, logger *log.Logger) (a.Payload, error) {
		b := []byte(`{"items":[{"id":"urn:ivcap:service:s1","score":0.2},{"id":"urn:ivcap:service:s2","score":0.9}]}`)
		return &fakePayload{b: b}, nil
	}

	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *log.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		resp := &aspect.ListResponseBody{}
		switch selector.Entity {
		case "urn:ivcap:service:s1":
			resp.Items = []*aspect.AspectListItemRTResponseBody{
				{Content: map[string]any{
					"name":        "tool_low",
					"description": "low",
					"fn_schema":   map[string]any{"type": "object", "properties": map[string]any{}},
					"service-id":  "urn:ivcap:service:s1",
				}},
			}
		case "urn:ivcap:service:s2":
			resp.Items = []*aspect.AspectListItemRTResponseBody{
				{Content: map[string]any{
					"name":        "tool_high",
					"description": "high",
					"fn_schema":   map[string]any{"type": "object", "properties": map[string]any{}},
					"service-id":  "urn:ivcap:service:s2",
				}},
			}
		}
		return resp, nil, nil
	}

	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)

	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	selMsg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"select_tools","arguments":{"interest":"x"}}}`)
	_ = s.HandleMessage(ctx, selMsg)

	listMsg := json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	out := s.HandleMessage(ctx, listMsg)
	res, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var parsed struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}
	if len(parsed.Tools) < 8 {
		t.Fatalf("expected at least 8 tools (select_tools + 5 built-ins + 2 selected), got %d", len(parsed.Tools))
	}
	if parsed.Tools[0].Name != "select_tools" {
		t.Fatalf("expected first tool to be select_tools, got %q", parsed.Tools[0].Name)
	}
	if parsed.Tools[1].Name != "artifact_create" {
		t.Fatalf("expected second tool to be artifact_create, got %q", parsed.Tools[1].Name)
	}
	if parsed.Tools[2].Name != "artifact_get" {
		t.Fatalf("expected third tool to be artifact_get, got %q", parsed.Tools[2].Name)
	}
	if parsed.Tools[3].Name != "aspect_search" {
		t.Fatalf("expected fourth tool to be aspect_search, got %q", parsed.Tools[3].Name)
	}
	if parsed.Tools[4].Name != "aspect_get" {
		t.Fatalf("expected fifth tool to be aspect_get, got %q", parsed.Tools[4].Name)
	}
	if parsed.Tools[5].Name != "aspect_create" {
		t.Fatalf("expected sixth tool to be aspect_create, got %q", parsed.Tools[5].Name)
	}
	if parsed.Tools[6].Name != "tool_high" {
		t.Fatalf("expected first selected tool to be tool_high (score 0.9), got %q", parsed.Tools[6].Name)
	}
	if parsed.Tools[7].Name != "tool_low" {
		t.Fatalf("expected second selected tool to be tool_low (score 0.2), got %q", parsed.Tools[7].Name)
	}
}
