// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
	aspect "github.com/ivcap-works/ivcap-core-api/http/aspect"
)

// genomeTestServer builds a server and restores the process-wide state afterwards.
func genomeTestServer(t *testing.T, cfg Config) *server.MCPServer {
	t.Helper()
	oldCfg, oldList, oldServices, oldRead := srvCfg, listAspectFn, listServicesRawFn, readServiceRawFn
	t.Cleanup(func() {
		srvCfg, listAspectFn, listServicesRawFn, readServiceRawFn = oldCfg, oldList, oldServices, oldRead
	})
	listAspectFn = func(ctx context.Context, selector sdk.AspectSelector, adpt *a.Adapter, logger *zap.Logger) (*aspect.ListResponseBody, a.Payload, error) {
		return &aspect.ListResponseBody{}, nil, nil
	}
	listServicesRawFn = func(ctxt context.Context, cmd *sdk.ListRequest, adpt *a.Adapter, logger *zap.Logger) (a.Payload, error) {
		return nil, nil
	}
	readServiceRawFn = func(ctxt context.Context, cmd *sdk.ReadServiceRequest, adpt *a.Adapter, logger *zap.Logger) (a.Payload, error) {
		return nil, nil
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.ToolSchema == "" {
		cfg.ToolSchema = "urn:sd-core:schema.ai-tool.1"
	}
	if cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = 1 // a deadline that any use of withTimeout would put on the context
	}
	if cfg.CreateAdapter == nil {
		cfg.CreateAdapter = func(int) (*a.Adapter, error) { return nil, nil }
	}
	return NewServer(cfg)
}

func genomeToolList(t *testing.T, s *server.MCPServer) []map[string]any {
	t.Helper()
	sess := server.NewInProcessSession("test", nil)
	ctx := s.WithContext(context.Background(), sess)
	_ = s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`))
	out := s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	res, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", out)
	}
	b, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.Tools
}

type genomeToolReply struct {
	text     string
	isError  bool // a tool result flagged as an error
	rpcError bool // a JSON-RPC error
}

func callGenomeTool(t *testing.T, s *server.MCPServer, name, args string) genomeToolReply {
	t.Helper()
	msg := json.RawMessage(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, args))
	switch out := s.HandleMessage(context.Background(), msg).(type) {
	case mcp.JSONRPCError:
		return genomeToolReply{rpcError: true, text: out.Error.Message}
	case mcp.JSONRPCResponse:
		b, err := json.Marshal(out.Result)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatal(err)
		}
		r := genomeToolReply{isError: parsed.IsError}
		if len(parsed.Content) > 0 {
			r.text = parsed.Content[0].Text
		}
		return r
	default:
		t.Fatalf("unexpected reply type %T", out)
		return genomeToolReply{}
	}
}

func TestGenomeToolsAreOfferedOnlyWithHooks(t *testing.T) {
	names := func(s *server.MCPServer) map[string]bool {
		got := map[string]bool{}
		for _, tool := range genomeToolList(t, s) {
			got[tool["name"].(string)] = true
		}
		return got
	}
	without := names(genomeTestServer(t, Config{}))
	if without["genome_import"] || without["genome_list"] {
		t.Errorf("the genome tools must not be offered without their hooks: %v", without)
	}
	with := names(genomeTestServer(t, Config{
		GenomeImport: func(context.Context, *a.Adapter, GenomeImportArgs) (any, error) { return nil, nil },
		GenomeList:   func(context.Context, *a.Adapter, GenomeListArgs) (any, error) { return nil, nil },
	}))
	if !with["genome_import"] || !with["genome_list"] {
		t.Errorf("the genome tools must be offered when the hooks are set: %v", with)
	}
	if len(with) != len(without)+2 {
		t.Errorf("%d tools with hooks, %d without: want exactly the two genome tools more", len(with), len(without))
	}
}

func TestGenomeListToolPassesArguments(t *testing.T) {
	var got GenomeListArgs
	s := genomeTestServer(t, Config{GenomeList: func(ctx context.Context, _ *a.Adapter, args GenomeListArgs) (any, error) {
		got = args
		return map[string]any{"genomes": []any{map[string]any{"accession": "GCF_1.1"}}}, nil
	}})
	r := callGenomeTool(t, s, "genome_list", `{"limit":5,"page":"abc"}`)
	if r.isError || r.rpcError {
		t.Fatalf("unexpected error: %+v", r)
	}
	if got != (GenomeListArgs{Limit: 5, Page: "abc"}) {
		t.Errorf("hook got %+v", got)
	}
	if !strings.Contains(r.text, "GCF_1.1") {
		t.Errorf("the hook's result must be returned, got %q", r.text)
	}
	got = GenomeListArgs{Limit: 99}
	if r := callGenomeTool(t, s, "genome_list", `{}`); r.isError || r.rpcError || got != (GenomeListArgs{}) {
		t.Errorf("no arguments is fine and must give zero values: %+v, got %+v", r, got)
	}
}

func TestGenomeImportToolPassesArgumentsAndHasNoDeadline(t *testing.T) {
	var got GenomeImportArgs
	var hadDeadline bool
	s := genomeTestServer(t, Config{GenomeImport: func(ctx context.Context, _ *a.Adapter, args GenomeImportArgs) (any, error) {
		got = args
		_, hadDeadline = ctx.Deadline()
		return map[string]any{"artifact": "urn:ivcap:artifact:x"}, nil
	}})
	r := callGenomeTool(t, s, "genome_import", `{"accession":" GCF_000001735.4 ","include":["genome","gff3"],"chromosomes":["chr1"],`+
		`"assembly_version":"current","name":"n.zip","policy":"urn:ivcap:policy:p","license":"CC0-1.0"}`)
	if r.isError || r.rpcError {
		t.Fatalf("unexpected error: %+v", r)
	}
	want := GenomeImportArgs{
		Accession: "GCF_000001735.4", Include: []string{"genome", "gff3"}, Chromosomes: []string{"chr1"},
		AssemblyVersion: "current", Name: "n.zip", Policy: "urn:ivcap:policy:p", License: "CC0-1.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hook got %+v, want %+v", got, want)
	}
	if hadDeadline {
		t.Error("an import must not get the CLI's --timeout as an overall deadline; it would cut a large import short")
	}
	if !strings.Contains(r.text, "urn:ivcap:artifact:x") {
		t.Errorf("the hook's result must be returned, got %q", r.text)
	}
}

func TestGenomeImportToolInputAndErrorHandling(t *testing.T) {
	called := false
	var hookErr error
	s := genomeTestServer(t, Config{GenomeImport: func(context.Context, *a.Adapter, GenomeImportArgs) (any, error) {
		called = true
		return nil, hookErr
	}})
	for _, args := range []string{`{}`, `{"accession":"   "}`} {
		if r := callGenomeTool(t, s, "genome_import", args); !r.rpcError || called {
			t.Errorf("%s: a missing accession must be rejected before the hook runs: %+v (called=%v)", args, r, called)
		}
	}
	hookErr = fmt.Errorf("NCBI said no")
	if r := callGenomeTool(t, s, "genome_import", `{"accession":"GCF_000001735.4"}`); !r.rpcError || !strings.Contains(r.text, "NCBI said no") {
		t.Errorf("an error of the hook must be passed on: %+v", r)
	}
	// an expired login is a tool error that tells the user what to do, not a protocol error
	called = false
	srvCfg.CreateAdapter = func(int) (*a.Adapter, error) { return nil, ErrLoginRequired }
	r := callGenomeTool(t, s, "genome_import", `{"accession":"GCF_000001735.4"}`)
	if !r.isError || !strings.Contains(r.text, "Authentication required") || called {
		t.Errorf("a login problem must be reported without running the import: %+v (called=%v)", r, called)
	}
}

func TestGenomeImportToolSchema(t *testing.T) {
	s := genomeTestServer(t, Config{
		GenomeImport: func(context.Context, *a.Adapter, GenomeImportArgs) (any, error) { return nil, nil },
		GenomeList:   func(context.Context, *a.Adapter, GenomeListArgs) (any, error) { return nil, nil },
	})
	for _, tool := range genomeToolList(t, s) {
		if tool["name"] != "genome_import" {
			continue
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		if req, _ := schema["required"].([]any); len(req) != 1 || req[0] != "accession" {
			t.Errorf("only accession is required, got %v", schema["required"])
		}
		props, _ := schema["properties"].(map[string]any)
		for _, p := range []string{"accession", "include", "chromosomes", "assembly_version", "name", "policy", "license"} {
			if _, ok := props[p]; !ok {
				t.Errorf("the schema lacks the property %q", p)
			}
		}
		if desc, _ := tool["description"].(string); !strings.Contains(desc, "WRITES") {
			t.Errorf("the description must warn that the tool writes to the platform: %q", desc)
		}
		return
	}
	t.Fatal("genome_import is not listed")
}

func TestGenomeImportToolExplainsItsOptions(t *testing.T) {
	s := genomeTestServer(t, Config{
		GenomeImport: func(context.Context, *a.Adapter, GenomeImportArgs) (any, error) { return nil, nil },
	})
	for _, tool := range genomeToolList(t, s) {
		if tool["name"] != "genome_import" {
			continue
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		props, _ := schema["properties"].(map[string]any)
		desc := func(p string) string {
			m, _ := props[p].(map[string]any)
			d, _ := m["description"].(string)
			return d
		}
		for _, want := range []string{"genome", "rna", "protein", "cds", "gff3", "gtf", "gbff", "seq-report", "all", "none", "FASTA", "annotation", "alone"} {
			if !strings.Contains(desc("include"), want) {
				t.Errorf("the description of include must explain %q: %s", want, desc("include"))
			}
		}
		for _, want := range []string{"current", "latest", "exactly one"} {
			if !strings.Contains(desc("assembly_version"), want) {
				t.Errorf("the description of assembly_version must mention %q: %s", want, desc("assembly_version"))
			}
		}
		versions, _ := props["assembly_version"].(map[string]any)["enum"].([]any)
		for _, v := range versions {
			if v == "all" {
				t.Error("'all' must not be offered as an assembly_version: an import needs exactly one version")
			}
		}
		if !strings.Contains(desc("accession"), "version") {
			t.Errorf("the description of accession must say that a version suffix picks a version: %s", desc("accession"))
		}
		return
	}
	t.Fatal("genome_import is not listed")
}

func hasName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// An agent that discovers tools through `search` must be led to the built-in genome tools, and not
// only to deployed services that happen to deal with genes.
func TestSearchAlsoFindsBuiltInTools(t *testing.T) {
	s := genomeTestServer(t, Config{
		GenomeImport: func(context.Context, *a.Adapter, GenomeImportArgs) (any, error) { return nil, nil },
		GenomeList:   func(context.Context, *a.Adapter, GenomeListArgs) (any, error) { return nil, nil },
	})
	search := func(query string) []string {
		r := callGenomeTool(t, s, "search", fmt.Sprintf(`{"tools":%q}`, query))
		if r.isError || r.rpcError {
			t.Fatalf("search %q failed: %+v", query, r)
		}
		var parsed struct {
			BuiltinTools []struct {
				Name string `json:"name"`
			} `json:"builtin_tools"`
		}
		if err := json.Unmarshal([]byte(r.text), &parsed); err != nil {
			t.Fatalf("search %q: not JSON: %q", query, r.text)
		}
		var names []string
		for _, b := range parsed.BuiltinTools {
			names = append(names, b.Name)
		}
		return names
	}

	got := search("which genomes are available on IVCAP")
	if !hasName(got, "genome_list") || !hasName(got, "genome_import") {
		t.Fatalf("both genome tools must be found, got %v", got)
	}
	if got[0] != "genome_list" {
		t.Errorf("the read-only tool must come before the one that writes, got %v", got)
	}
	if hasName(got, "service_list") || hasName(got, "artifact_create") {
		t.Errorf("tools that do not match the question must not be returned: %v", got)
	}
	if got := search("artifact"); !hasName(got, "artifact_get") || !hasName(got, "artifact_create") {
		t.Errorf("other built-in tools must be found too, got %v", got)
	}
	if got := search("what is available"); len(got) != 0 {
		t.Errorf("a question made of stop words must not match every tool, got %v", got)
	}

	// without the hooks there are no genome tools, so none can be found
	s = genomeTestServer(t, Config{})
	if got := search("genomes"); hasName(got, "genome_list") {
		t.Errorf("a server without the genome hooks must not offer them in a search: %v", got)
	}
}

func TestSearchToolDescriptionMentionsBuiltInTools(t *testing.T) {
	s := genomeTestServer(t, Config{})
	for _, tool := range genomeToolList(t, s) {
		if tool["name"] == "search" {
			if d, _ := tool["description"].(string); !strings.Contains(d, "builtin_tools") {
				t.Errorf("the description of search must explain builtin_tools: %q", d)
			}
			return
		}
	}
	t.Fatal("search is not listed")
}

func TestServerInstructions(t *testing.T) {
	instructions := func(cfg Config) string {
		s := genomeTestServer(t, cfg)
		sess := server.NewInProcessSession("test", nil)
		ctx := s.WithContext(context.Background(), sess)
		out := s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`))
		res, ok := out.(mcp.JSONRPCResponse)
		if !ok {
			t.Fatalf("expected JSONRPCResponse, got %T", out)
		}
		b, err := json.Marshal(res.Result)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Instructions string `json:"instructions"`
		}
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatal(err)
		}
		return parsed.Instructions
	}

	with := instructions(Config{
		GenomeImport: func(context.Context, *a.Adapter, GenomeImportArgs) (any, error) { return nil, nil },
		GenomeList:   func(context.Context, *a.Adapter, GenomeListArgs) (any, error) { return nil, nil },
	})
	for _, want := range []string{"genome_list", "genome_import", "which genomes are available", "read_skill", "skills='*'", "builtin_tools", "confirm"} {
		if !strings.Contains(with, want) {
			t.Errorf("the instructions must mention %q:\n%s", want, with)
		}
	}
	without := instructions(Config{})
	if strings.Contains(without, "genome") {
		t.Errorf("without the genome hooks the instructions must not talk about genomes:\n%s", without)
	}
	if !strings.Contains(without, "read_skill") {
		t.Errorf("the instructions must always say how to read skills without MCP resources:\n%s", without)
	}
}

func TestGenomeListToolRoutesQuestionsAboutAvailableGenomes(t *testing.T) {
	s := genomeTestServer(t, Config{
		GenomeList: func(context.Context, *a.Adapter, GenomeListArgs) (any, error) { return nil, nil },
	})
	for _, tool := range genomeToolList(t, s) {
		if tool["name"] != "genome_list" {
			continue
		}
		d, _ := tool["description"].(string)
		for _, want := range []string{"available", "imported", "which genomes are available on IVCAP", "organism", "assembly_name", "does not search NCBI", "not the service search"} {
			if !strings.Contains(d, want) {
				t.Errorf("the description of genome_list must contain %q: %s", want, d)
			}
		}
		return
	}
	t.Fatal("genome_list is not listed")
}

func skillNames(t *testing.T, s *server.MCPServer, args string) []string {
	t.Helper()
	r := callGenomeTool(t, s, "search", args)
	if r.isError || r.rpcError {
		t.Fatalf("search %s failed: %+v", args, r)
	}
	var parsed struct {
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(r.text), &parsed); err != nil {
		t.Fatalf("search %s: not JSON: %q", args, r.text)
	}
	var names []string
	for _, sk := range parsed.Skills {
		names = append(names, sk.Name)
	}
	return names
}

// Skills must be findable without MCP resources: by a keyword, by the exact name, and by a question.
func TestSkillsSearchUnderstandsQuestionsAndNames(t *testing.T) {
	s := genomeTestServer(t, Config{})

	got := skillNames(t, s, `{"skills":"which genomes are available"}`)
	if len(got) == 0 || got[0] != "ivcap-genome-list" {
		t.Errorf("a question about available genomes must find ivcap-genome-list first, got %v", got)
	}
	if got := skillNames(t, s, `{"skills":"ivcap-genome-import"}`); len(got) == 0 || got[0] != "ivcap-genome-import" {
		t.Errorf("an exact skill name must come first, got %v", got)
	}
	got = skillNames(t, s, `{"skills":"genome"}`)
	for _, want := range []string{"ivcap-genome-list", "ivcap-genome-import", "ivcap-genome-download"} {
		if !hasName(got, want) {
			t.Errorf("the keyword genome must find %s, got %v", want, got)
		}
	}
	docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
	if err != nil {
		t.Fatal(err)
	}
	if got := skillNames(t, s, `{"skills":"*"}`); len(got) != len(docs) || !hasName(got, "ivcap-genome-list") {
		t.Errorf("skills='*' must list all %d skills, not just the first 10, got %d: %v", len(docs), len(got), got)
	}
	if got := skillNames(t, s, `{"skills":"*","limit":3}`); len(got) != 3 {
		t.Errorf("an explicit limit must also apply to skills='*', got %v", got)
	}
	if got := skillNames(t, s, `{"skills":"genome","limit":1}`); len(got) != 1 {
		t.Errorf("the limit must apply, got %v", got)
	}
	if got := skillNames(t, s, `{"skills":"zzzznothing"}`); len(got) != 0 {
		t.Errorf("a query that matches nothing must return no skills, got %v", got)
	}
}

// list_skills was folded into the search tool; nothing may still tell an agent to call it.
func TestNoToolDescriptionPointsAtTheRemovedListSkillsTool(t *testing.T) {
	s := genomeTestServer(t, Config{})
	for _, tool := range genomeToolList(t, s) {
		if d, _ := tool["description"].(string); strings.Contains(d, "list_skills") {
			t.Errorf("the description of %v still mentions list_skills: %s", tool["name"], d)
		}
		if tool["name"] == "read_skill" {
			if d, _ := tool["description"].(string); !strings.Contains(d, "search") {
				t.Errorf("read_skill must say how to find the exact name (the search tool): %s", d)
			}
		}
	}
}
