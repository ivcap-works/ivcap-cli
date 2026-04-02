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
