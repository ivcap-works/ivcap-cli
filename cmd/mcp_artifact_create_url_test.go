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
