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

func TestMCPAspectCreate_InjectsSchemaAndCallsAdd(t *testing.T) {
	oldAdd := addUpdateAspectFn
	oldAdapter := createMCPAdapterFn
	t.Cleanup(func() {
		addUpdateAspectFn = oldAdd
		createMCPAdapterFn = oldAdapter
	})

	createMCPAdapterFn = func(timeoutSec int) (*a.Adapter, error) { return nil, nil }
	called := false
	addUpdateAspectFn = func(ctxt context.Context, isAdd bool, entity string, schema string, policy string, meta []byte, adpt *a.Adapter, logger *zap.Logger) (a.Payload, error) {
		called = true
		if !isAdd {
			t.Fatalf("expected isAdd=true")
		}
		if entity != "urn:ivcap:entity:1" {
			t.Fatalf("unexpected entity: %q", entity)
		}
		if schema != "urn:ivcap:schema:test" {
			t.Fatalf("unexpected schema: %q", schema)
		}
		var obj map[string]any
		if err := json.Unmarshal(meta, &obj); err != nil {
			t.Fatalf("invalid json body: %v", err)
		}
		if obj["$schema"] != "urn:ivcap:schema:test" {
			t.Fatalf("expected $schema to be injected, got: %+v", obj["$schema"])
		}
		return &fakePayload{b: []byte(`{"id":"urn:ivcap:aspect:new"}`)}, nil
	}

	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)
	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"aspect_create","arguments":{"entity":"urn:ivcap:entity:1","schema":"urn:ivcap:schema:test","body":{"hello":"world"}}}}`)
	out := s.HandleMessage(ctx, msg)
	if _, ok := out.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	if !called {
		t.Fatalf("expected addUpdateAspectFn to be called")
	}
}
