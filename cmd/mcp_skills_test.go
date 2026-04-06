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
)

func TestMCPResources_SkillsManifestAndSkillBody(t *testing.T) {
	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)

	// initialize session
	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	// resources/list should contain the skill resources
	listMsg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}`)
	out := s.HandleMessage(ctx, listMsg)
	resp, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var listed struct {
		Resources []mcp.Resource `json:"resources"`
	}
	if err := json.Unmarshal(b, &listed); err != nil {
		t.Fatalf("cannot unmarshal resources/list result: %v", err)
	}
	got := map[string]bool{}
	for _, r := range listed.Resources {
		got[r.URI] = true
	}
	if !got["skills://manifest"] || !got["skills://catalog.json"] || !got["skills://CONTEXT.md"] || !got["skills://SKILLS.md"] {
		t.Fatalf("expected skills resources to be listed; got %+v", got)
	}

	// resources/read manifest
	readManifest := json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"skills://manifest"}}`)
	out = s.HandleMessage(ctx, readManifest)
	resp, ok = out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err = json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var manifestRes struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(b, &manifestRes); err != nil {
		t.Fatalf("cannot unmarshal resources/read manifest result: %v", err)
	}
	if len(manifestRes.Contents) != 1 {
		t.Fatalf("expected 1 manifest content item, got %d", len(manifestRes.Contents))
	}
	if manifestRes.Contents[0].MIMEType != "application/json" {
		t.Fatalf("expected application/json, got %q", manifestRes.Contents[0].MIMEType)
	}
	var parsedManifest struct {
		Skills []struct {
			Name string `json:"name"`
			URI  string `json:"uri"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(manifestRes.Contents[0].Text), &parsedManifest); err != nil {
		t.Fatalf("cannot parse manifest JSON: %v", err)
	}
	var found bool
	for _, it := range parsedManifest.Skills {
		if it.Name == "ivcap-job-create" {
			found = true
			if it.URI != "skills://ivcap-job-create/SKILL.md" {
				t.Fatalf("unexpected ivcap-job-create URI: %q", it.URI)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ivcap-job-create skill in manifest; got %v items", len(parsedManifest.Skills))
	}

	// resources/read one skill body via template
	readSkill := json.RawMessage(`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"skills://ivcap-job-create/SKILL.md"}}`)
	out = s.HandleMessage(ctx, readSkill)
	resp, ok = out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err = json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var skillRes struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(b, &skillRes); err != nil {
		t.Fatalf("cannot unmarshal resources/read skill result: %v", err)
	}
	if len(skillRes.Contents) != 1 {
		t.Fatalf("expected 1 skill content item, got %d", len(skillRes.Contents))
	}
	if skillRes.Contents[0].MIMEType != "text/markdown" {
		t.Fatalf("expected text/markdown, got %q", skillRes.Contents[0].MIMEType)
	}
	if len(skillRes.Contents[0].Text) < 4 || skillRes.Contents[0].Text[:3] != "---" {
		t.Fatalf("expected skill markdown to start with YAML front-matter delimiter")
	}

	// resources/read skills index
	readIndex := json.RawMessage(`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"skills://SKILLS.md"}}`)
	out = s.HandleMessage(ctx, readIndex)
	resp, ok = out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err = json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var indexRes struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(b, &indexRes); err != nil {
		t.Fatalf("cannot unmarshal resources/read index result: %v", err)
	}
	if len(indexRes.Contents) != 1 {
		t.Fatalf("expected 1 index content item, got %d", len(indexRes.Contents))
	}
	if indexRes.Contents[0].MIMEType != "text/markdown" {
		t.Fatalf("expected text/markdown, got %q", indexRes.Contents[0].MIMEType)
	}
	if indexRes.Contents[0].Text == "" {
		t.Fatalf("expected non-empty index markdown")
	}

	// resources/read a category SKILLS.md via file template
	readCat := json.RawMessage(`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"skills://file/service/SKILLS.md"}}`)
	out = s.HandleMessage(ctx, readCat)
	resp, ok = out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err = json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var catRes struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(b, &catRes); err != nil {
		t.Fatalf("cannot unmarshal resources/read category result: %v", err)
	}
	if len(catRes.Contents) != 1 {
		t.Fatalf("expected 1 category content item, got %d", len(catRes.Contents))
	}
	if catRes.Contents[0].URI != "skills://file/service/SKILLS.md" {
		t.Fatalf("unexpected category uri: %q", catRes.Contents[0].URI)
	}
	if catRes.Contents[0].Text == "" {
		t.Fatalf("expected non-empty category markdown")
	}
}

func TestMCPPrompts_UseIvcapBestPractices(t *testing.T) {
	s := newCLIMCPServer()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)

	initMsg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	_ = s.HandleMessage(ctx, initMsg)

	listMsg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"prompts/list","params":{}}`)
	out := s.HandleMessage(ctx, listMsg)
	resp, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var listed struct {
		Prompts []mcp.Prompt `json:"prompts"`
	}
	if err := json.Unmarshal(b, &listed); err != nil {
		t.Fatalf("cannot unmarshal prompts/list result: %v", err)
	}
	var found bool
	for _, p := range listed.Prompts {
		if p.Name == "use-ivcap-best-practices" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected use-ivcap-best-practices prompt to be listed")
	}

	getMsg := json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"use-ivcap-best-practices"}}`)
	out = s.HandleMessage(ctx, getMsg)
	resp, ok = out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err = json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("cannot marshal result: %v", err)
	}
	var got struct {
		Messages []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("cannot unmarshal prompts/get result: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 prompt message, got %d", len(got.Messages))
	}
	if got.Messages[0].Role != "user" {
		t.Fatalf("expected role user, got %q", got.Messages[0].Role)
	}
	if got.Messages[0].Content.Type != "text" {
		t.Fatalf("expected content type text, got %q", got.Messages[0].Content.Type)
	}
	if got.Messages[0].Content.Text == "" {
		t.Fatalf("expected non-empty prompt text")
	}
}
