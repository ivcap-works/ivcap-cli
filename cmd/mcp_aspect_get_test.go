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

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

func TestMCPAspectGet_UsesGetAspectRaw(t *testing.T) {
	oldGet := getAspectRawFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		getAspectRawFn = oldGet
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }
	called := false
	getAspectRawFn = func(ctxt context.Context, recordID string, adpt *a.Adapter, logger *zap.Logger) (a.Payload, error) {
		called = true
		if recordID != "urn:ivcap:aspect:123" {
			t.Fatalf("unexpected id: %q", recordID)
		}
		return &fakePayload{b: []byte(`{"id":"urn:ivcap:aspect:123"}`)}, nil
	}

	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)
	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"aspect_get","arguments":{"id":"urn:ivcap:aspect:123"}}}`)
	out := s.HandleMessage(ctx, msg)
	if _, ok := out.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	if !called {
		t.Fatalf("expected getAspectRawFn to be called")
	}
}
