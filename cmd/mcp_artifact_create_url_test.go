package cmd

import (
	"context"
	"testing"
)

func TestMcpPartToBytes_URLSource(t *testing.T) {
	defer func(old func(context.Context, string) ([]byte, string, error)) { fetchURLBytesFn = old }(fetchURLBytesFn)
	fetchURLBytesFn = func(ctx context.Context, u string) ([]byte, string, error) {
		if u != "https://example.com/file.pdf" {
			t.Fatalf("unexpected url: %s", u)
		}
		return []byte("pdfbytes"), "application/pdf", nil
	}

	p := mcpContentPart{Type: "document", Source: &struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data,omitempty"`
		URL       string `json:"url,omitempty"`
	}{Type: "url", URL: "https://example.com/file.pdf"}}

	b, mt, err := mcpPartToBytes(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(b) != "pdfbytes" {
		t.Fatalf("unexpected bytes: %q", string(b))
	}
	if mt != "application/pdf" {
		t.Fatalf("unexpected mt: %q", mt)
	}
}
