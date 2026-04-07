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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

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
	oldGet := getAspectRawFn
	oldAdapter := srvCfg.CreateAdapter
	t.Cleanup(func() {
		getAspectRawFn = oldGet
		srvCfg.CreateAdapter = oldAdapter
	})

	srvCfg.CreateAdapter = func(timeoutSec int) (*a.Adapter, error) {
		// returning a nil adapter is ok because listAspectFn won't use it
		return nil, nil
	}
	getAspectRawFn = func(ctxt context.Context, recordID string, adpt *a.Adapter, logger *zap.Logger) (a.Payload, error) {
		return nil, &a.UnauthorizedError{}
	}

	s := NewServer(Config{Logger: zap.NewNop(), ToolSchema: "urn:sd-core:schema.ai-tool.1", TimeoutSec: 1, CreateAdapter: srvCfg.CreateAdapter})
	msg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"aspect_get","arguments":{"id":"urn:ivcap:aspect:1"}}}`)
	out := s.HandleMessage(context.Background(), msg)

	errResp, ok := out.(mcp.JSONRPCError)
	if !ok {
		t.Fatalf("expected JSONRPCError, got %T", out)
	}
	if errResp.Error.Message != LoginRequiredMessage {
		t.Fatalf("expected message %q, got %q", LoginRequiredMessage, errResp.Error.Message)
	}
}

func TestMCPToolsList_InitiallyHasBuiltins(t *testing.T) {
	old := listAspectFn
	oldListServices := listServicesRawFn
	oldAdapter := srvCfg.CreateAdapter
	t.Cleanup(func() {
		listAspectFn = old
		listServicesRawFn = oldListServices
		srvCfg.CreateAdapter = oldAdapter
	})

	srvCfg.CreateAdapter = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }
	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *zap.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		// No tools returned from platform. That's fine.
		return &aspect.ListResponseBody{}, nil, nil
	}
	listServicesRawFn = func(ctxt context.Context, cmd *sdk.ListRequest, adpt *a.Adapter, logger *zap.Logger) (a.Payload, error) {
		return nil, nil
	}

	s := NewServer(Config{Logger: zap.NewNop(), ToolSchema: "urn:sd-core:schema.ai-tool.1", TimeoutSec: 1, CreateAdapter: srvCfg.CreateAdapter})

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
	if len(parsed.Tools) != 8 {
		t.Fatalf("expected 8 tools initially, got %d", len(parsed.Tools))
	}
	if parsed.Tools[0].Name != "select_tools" {
		t.Fatalf("expected first tool to be select_tools, got %q", parsed.Tools[0].Name)
	}
	got := map[string]bool{}
	for _, t0 := range parsed.Tools {
		got[t0.Name] = true
	}
	if !got["select_tools"] || !got["artifact_create"] || !got["artifact_get"] || !got["aspect_search"] || !got["aspect_get"] || !got["aspect_create"] || !got["nextflow_create"] || !got["nextflow_run"] {
		t.Fatalf("expected built-in tools; got %+v", got)
	}
}
