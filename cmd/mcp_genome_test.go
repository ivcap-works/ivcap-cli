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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	mcppkg "github.com/ivcap-works/ivcap-cli/pkg/mcp"
)

// captureStdout runs fn and returns everything it wrote to os.Stdout. In MCP's STDIO mode stdout
// carries the protocol, so the genome tools must never print to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	os.Stdout = old
	_ = w.Close()
	return <-done
}

func TestMcpGenomeImport(t *testing.T) {
	f := newImportFixture(t)
	var res any
	var err error
	stdout := captureStdout(t, func() {
		res, err = mcpGenomeImportWith(context.Background(), ncbiTestClient(f.srv), &f.adapter,
			mcppkg.GenomeImportArgs{Accession: "GCF_000001735.4"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Errorf("the tool must not write to stdout, which carries the MCP protocol: %q", stdout)
	}
	result, ok := res.(genomeImportOutput)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	if result.Artifact != importTestArtifact || result.Accession != "GCF_000001735.4" ||
		result.ReportAspect != importTestAspect1 || result.ManifestAspect == "" {
		t.Errorf("unexpected result: %+v", result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.aspects) != 2 {
		t.Fatalf("want a report and a manifest aspect, got %d aspects", len(f.aspects))
	}
	graph, _ := f.aspects[1]["ro_crate"].(map[string]any)["@graph"].([]any)
	root, _ := graph[1].(map[string]any)
	if lic, _ := root["license"].(map[string]any); lic["@id"] != ncbiDataLicense.ID {
		t.Errorf("without a license argument the manifest must link to NCBI's data policy, got %v", root["license"])
	}
}

func TestMcpGenomeImportLicenseArgument(t *testing.T) {
	f := newImportFixture(t)
	if _, err := mcpGenomeImportWith(context.Background(), ncbiTestClient(f.srv), &f.adapter,
		mcppkg.GenomeImportArgs{Accession: "GCF_000001735.4", Include: []string{"genome", "gff3"}, License: "CC0-1.0"}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	graph, _ := f.aspects[1]["ro_crate"].(map[string]any)["@graph"].([]any)
	root, _ := graph[1].(map[string]any)
	if lic, _ := root["license"].(map[string]any); lic["@id"] != "https://spdx.org/licenses/CC0-1.0" {
		t.Errorf("the license argument must reach the manifest, got %v", root["license"])
	}
}

func TestMcpGenomeImportRejectsBadInputBeforeAnyRequest(t *testing.T) {
	for name, in := range map[string]mcppkg.GenomeImportArgs{
		"BioProject":      {Accession: "PRJNA10719"},
		"invalid license": {Accession: "GCF_000001735.4", License: "not a license!"},
		"invalid include": {Accession: "GCF_000001735.4", Include: []string{"nonsense"}},
	} {
		f := newImportFixture(t)
		if _, err := mcpGenomeImportWith(context.Background(), ncbiTestClient(f.srv), &f.adapter, in); err == nil {
			t.Errorf("%s: want an error", name)
		}
		if n := f.total(); n != 0 {
			t.Errorf("%s: nothing may be requested, got %d requests", name, n)
		}
	}
}

func TestMcpGenomeList(t *testing.T) {
	f := newListFixture(t, listTestItems(), "cursor-123")
	var res any
	var err error
	stdout := captureStdout(t, func() {
		res, err = mcpGenomeList(context.Background(), &f.adapter, mcppkg.GenomeListArgs{Limit: 7})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Errorf("the tool must not write to stdout, which carries the MCP protocol: %q", stdout)
	}
	out, ok := res.(genomeListOutput)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	if len(out.Genomes) != 3 || out.NextPage != "cursor-123" {
		t.Errorf("unexpected result: %+v", out)
	}
	if len(history) != 0 {
		t.Errorf("a tool call must not add to the CLI's @n history: %v", history)
	}
	b, _ := json.Marshal(res)
	for _, key := range []string{`"genomes"`, `"accession"`, `"organism"`, `"assembly_name"`, `"report_aspect"`, `"artifact"`, `"next_page"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("the JSON result lacks %s: %s", key, b)
		}
	}
}

func TestMcpGenomeImportPolicies(t *testing.T) {
	f := newImportFixture(t)
	if _, err := mcpGenomeImportWith(context.Background(), ncbiTestClient(f.srv), &f.adapter,
		mcppkg.GenomeImportArgs{Accession: "GCF_000001735.4"}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	if f.artifactPolicy != genomeOpenArtifactPolicy || len(f.aspectPolicies) != 2 || f.aspectPolicies[0] != genomeOpenAspectPolicy {
		t.Errorf("without a policy argument the open policies apply, got %q and %v", f.artifactPolicy, f.aspectPolicies)
	}
	f.mu.Unlock()

	f = newImportFixture(t)
	if _, err := mcpGenomeImportWith(context.Background(), ncbiTestClient(f.srv), &f.adapter,
		mcppkg.GenomeImportArgs{Accession: "GCF_000001735.4", Policy: "urn:ivcap:policy:custom"}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.artifactPolicy != "urn:ivcap:policy:custom" || f.aspectPolicies[0] != "urn:ivcap:policy:custom" {
		t.Errorf("a policy argument must override the open policies, got %q and %v", f.artifactPolicy, f.aspectPolicies)
	}
}
