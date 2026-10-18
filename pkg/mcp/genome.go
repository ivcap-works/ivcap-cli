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
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tools: genome_import and genome_list
//
// The genome logic lives in cmd/ next to the 'ivcap genome' commands, and pkg/mcp does not depend on
// cmd/. So it is injected through Config.GenomeImport and Config.GenomeList, and a tool is only
// offered when its hook is set.

// GenomeImportArgs are the arguments of the genome_import tool.
type GenomeImportArgs struct {
	Accession       string   `json:"accession"`
	Include         []string `json:"include,omitempty"`
	Chromosomes     []string `json:"chromosomes,omitempty"`
	AssemblyVersion string   `json:"assembly_version,omitempty"`
	Name            string   `json:"name,omitempty"`
	Policy          string   `json:"policy,omitempty"`
	License         string   `json:"license,omitempty"`
}

// GenomeListArgs are the arguments of the genome_list tool.
type GenomeListArgs struct {
	Limit int    `json:"limit,omitempty"`
	Page  string `json:"page,omitempty"`
}

// GenomeImportFunc imports one NCBI genome into IVCAP and returns a JSON-serialisable result.
// Nothing may be written to stdout: in STDIO mode it carries the MCP protocol.
type GenomeImportFunc func(ctxt context.Context, adapter *a.Adapter, args GenomeImportArgs) (any, error)

// GenomeListFunc lists the genomes imported into IVCAP and returns a JSON-serialisable result.
type GenomeListFunc func(ctxt context.Context, adapter *a.Adapter, args GenomeListArgs) (any, error)

func addGenomeListTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max number of genomes to return.",
				"minimum":     1,
			},
			"page": map[string]any{
				"type":        "string",
				"description": "Paging cursor: the next_page value of a previous result.",
			},
		},
	}
	tool := mcp.NewToolWithRawSchema(
		"genome_list",
		"List the genomes that are available on this IVCAP platform because they were imported with genome_import. Use this "+
			"tool, not the service search, to answer questions such as 'which genomes are available on IVCAP', 'what organisms "+
			"or assemblies are stored' or 'is genome X already imported'. Each entry has the accession, the organism and "+
			"assembly_name from the assembly report, the report_aspect and the artifact holding the imported package. "+
			"Read-only. It lists what is already on the platform and does not search NCBI. If the result has next_page, "+
			"pass it as page to get more.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			args = map[string]any{}
		}
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed GenomeListArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}
		if srvCfg.GenomeList == nil {
			return nil, fmt.Errorf("genome_list is not available")
		}
		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		result, err := srvCfg.GenomeList(ctxt, adpt, parsed)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		return mcp.NewToolResultJSON(result)
	}
	s.AddTool(tool, handler)
}

func addGenomeImportTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"accession": map[string]any{
				"type":        "string",
				"description": "Assembly accession (GCF_... for RefSeq, GCA_... for GenBank) of exactly one genome. A version suffix such as GCF_000001735.4 picks that version. BioProject accessions (PRJNA...) are rejected.",
			},
			"include": map[string]any{
				"type":        "array",
				"description": "Files to include in the package (default: genome). genome = genomic sequence (FASTA); rna = transcript sequences; protein = amino acid sequences; cds = nucleotide coding sequences; gff3 and gtf = annotation in that format; gbff = GenBank flat file; seq-report = per-sequence report; all = every type above (use alone); none = metadata only (use alone).",
				"items": map[string]any{
					"type": "string",
					"enum": []any{"genome", "rna", "protein", "cds", "gff3", "gtf", "gbff", "seq-report", "all", "none"},
				},
			},
			"chromosomes": map[string]any{
				"type":        "array",
				"description": "Limit the genome FASTA to these chromosomes, or ['all']. The names are passed to NCBI unchanged.",
				"items":       map[string]any{"type": "string"},
			},
			"assembly_version": map[string]any{
				"type":        "string",
				"description": "Omit to use the accession as given: a versioned accession such as GCF_000001735.4 is used as is, an unversioned one resolves to the current version. 'current' (same as 'latest') ignores any version in the accession and takes the current one. ('all' is not offered: an import needs exactly one version.)",
				"enum":        []any{"current", "latest"},
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Artifact name. Default: <accession>.zip.",
			},
			"policy": map[string]any{
				"type":        "string",
				"description": "Policy controlling access to the artifact and its aspects. By default the open policies are used, because the imported data is public NCBI data. Only set this to restrict access, or if the user gave one.",
			},
			"license": map[string]any{
				"type": "string",
				"description": "License of the data, recorded in the manifest: an SPDX identifier such as CC0-1.0, a license URL, " +
					"or 'undefined'. By default the manifest links to NCBI's data usage policy, which states no license. " +
					"Only set this if the user states a license.",
			},
		},
		"required": []any{"accession"},
	}
	tool := mcp.NewToolWithRawSchema(
		"genome_import",
		"Download the data package of one NCBI genome and import it into IVCAP as an artifact (a zip file) with an assembly "+
			"report aspect and an RO-Crate manifest aspect. This WRITES to the platform: confirm with the user first. Large "+
			"genomes can take several minutes, and importing the same accession again creates a new artifact. Returns the "+
			"artifact, accession, bytes, report_aspect and manifest_aspect. Use genome_list to see what has been imported.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pyld, err := a.JsonPayloadFromAny(req.GetArguments(), srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed GenomeImportArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}
		parsed.Accession = strings.TrimSpace(parsed.Accession)
		if parsed.Accession == "" {
			return nil, fmt.Errorf("accession is required")
		}
		if srvCfg.GenomeImport == nil {
			return nil, fmt.Errorf("genome_import is not available")
		}
		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		// No overall deadline from the CLI's --timeout: downloading and uploading a genome can take
		// minutes, and stopping it half way would leave an incomplete artifact behind. The client
		// ends the call by cancelling the request.
		ctxt, cancel := context.WithCancel(ctx)
		defer cancel()

		result, err := srvCfg.GenomeImport(ctxt, adpt, parsed)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		return mcp.NewToolResultJSON(result)
	}
	s.AddTool(tool, handler)
}
