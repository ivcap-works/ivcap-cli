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

package cmd

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	aspect "github.com/ivcap-works/ivcap-core-api/http/aspect"
)

func TestMCPAspectSearch_PassesIncludeContentAndFilters(t *testing.T) {
	old := listAspectFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		listAspectFn = old
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }

	called := false
	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *zap.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		called = true
		if selector.Entity != "urn:ivcap:entity:1" {
			t.Fatalf("unexpected entity: %q", selector.Entity)
		}
		if selector.SchemaPrefix != "urn:ivcap:schema:test" {
			t.Fatalf("unexpected schema prefix: %q", selector.SchemaPrefix)
		}
		if !selector.IncludeContent {
			t.Fatalf("expected IncludeContent=true")
		}
		if selector.JsonFilter == nil || *selector.JsonFilter != "$.x" {
			t.Fatalf("unexpected JsonFilter: %+v", selector.JsonFilter)
		}
		if selector.ListRequest.Limit != 7 {
			t.Fatalf("unexpected limit: %d", selector.ListRequest.Limit)
		}
		resp := &aspect.ListResponseBody{}
		return resp, &fakePayload{b: []byte(`{"items":[]}`)}, nil
	}

	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)
	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"aspect_search","arguments":{"entity":"urn:ivcap:entity:1","schema_prefix":"urn:ivcap:schema:test","include_content":true,"content_path":"$.x","limit":7}}}`)
	out := s.HandleMessage(ctx, msg)
	if _, ok := out.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	if !called {
		t.Fatalf("expected listAspectFn to be called")
	}
}
