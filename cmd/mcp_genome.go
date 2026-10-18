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
	"fmt"
	"io"
	"strings"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	mcppkg "github.com/ivcap-works/ivcap-cli/pkg/mcp"
	ncbigenome "github.com/ivcap-works/ivcap-ncbi-genome-download"
)

// The MCP tools genome_import and genome_list run the same code as 'ivcap genome import' and
// 'ivcap genome list'. In STDIO mode stdout carries the MCP protocol, so nothing here may print
// to it: both run in their machine-readable, silent mode and their output is parsed back.

var (
	_ mcppkg.GenomeImportFunc = mcpGenomeImport
	_ mcppkg.GenomeListFunc   = mcpGenomeList
)

func mcpGenomeImport(ctxt context.Context, adapter *adpt.Adapter, in mcppkg.GenomeImportArgs) (any, error) {
	return mcpGenomeImportWith(ctxt, ncbiNewClient("", true, io.Discard), adapter, in)
}

func mcpGenomeImportWith(
	ctxt context.Context,
	client *ncbigenome.Client,
	adapter *adpt.Adapter,
	in mcppkg.GenomeImportArgs,
) (any, error) {
	includes := in.Include
	if len(includes) == 0 {
		includes = []string{"genome"} // the default of 'genome import --include'
	}
	o := genomeImportOpts{
		Accession:       strings.TrimSpace(in.Accession),
		Includes:        includes,
		Chromosomes:     in.Chromosomes,
		AssemblyVersion: in.AssemblyVersion,
		Name:            in.Name,
		Policy:          in.Policy,
		ChunkSize:       DEF_CHUNK_SIZE,
		ReportSchema:    ncbiAssemblyReportSchema,
		ManifestSchema:  artifactArchiveManifestSchema,
		License:         in.License,
		Format:          "json",
		Silent:          true,
	}
	var out bytes.Buffer
	if err := runGenomeImport(ctxt, client, o, adapter, &out, io.Discard); err != nil {
		return nil, err
	}
	var result genomeImportOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("unexpected import result: %w", err)
	}
	return result, nil
}

func mcpGenomeList(ctxt context.Context, adapter *adpt.Adapter, in mcppkg.GenomeListArgs) (any, error) {
	req := sdk.ListRequest{Limit: DEF_LIMIT}
	if in.Limit > 0 {
		req.Limit = in.Limit
	}
	if in.Page != "" {
		req.Page = &in.Page
	}
	var out bytes.Buffer
	if err := runGenomeList(ctxt, adapter, ncbiAssemblyReportSchema, req, "json", &out); err != nil {
		return nil, err
	}
	var result genomeListOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("unexpected list result: %w", err)
	}
	return result, nil
}
