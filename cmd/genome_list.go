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
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/aspect"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

// 'genome list' shows the genomes that 'genome import' stored: it asks the data fabric for the
// assembly report aspects and takes the artifact from the entity each of them is attached to.

var (
	genomeListSchema string

	genomeListCmd = &cobra.Command{
		Use:   "list [flags]",
		Short: "List the genomes imported into IVCAP",
		Long: `List the genomes that 'ivcap genome import' stored in IVCAP, one row per import.

A genome is listed when the data fabric holds an assembly report aspect for it. Each row shows the
accession from that report, the report aspect, and the artifact the aspect belongs to, which holds the
imported package. Importing an accession twice gives two rows. Use --report-schema if the imports used
another report schema.

The report aspect and the artifact are shown as short references such as @3, which other commands accept in
place of the URN. Use --no-history to show the full URNs instead.

The paging, ordering and time flags are the same as for the other list commands. A last row such as
"... @5" is the reference to the next page: pass it to --page.

With --output json|yaml the same rows, with the organism and assembly name of each report, are printed as 'genomes', plus 'next_page' when there are more
results; pass that to --page to continue.`,
		Example: `  ivcap genome list
  ivcap --output json genome list --limit 50`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenomeList(context.Background(), CreateAdapter(true), genomeListSchema,
				*createListRequest(), outputFormat, os.Stdout)
		},
	}
)

func init() {
	genomeCmd.AddCommand(genomeListCmd)
	addListFlags(genomeListCmd)
	genomeListCmd.Flags().StringVar(&genomeListSchema, "report-schema", ncbiAssemblyReportSchema,
		"Schema URN of the assembly report aspects to list")
}

const genomeUnknownAccession = "(unknown)"

type genomeListRow struct {
	Accession    string `json:"accession"`
	Organism     string `json:"organism,omitempty"`
	AssemblyName string `json:"assembly_name,omitempty"`
	ReportAspect string `json:"report_aspect"`
	Artifact     string `json:"artifact"`
}

type genomeListOutput struct {
	Genomes  []genomeListRow `json:"genomes"`
	AtTime   string          `json:"at_time,omitempty"`
	NextPage string          `json:"next_page,omitempty"`

	nextLink *string // the API's link to the next page; the table's next-page row is built from it
}

func runGenomeList(
	ctxt context.Context,
	adapter *adpt.Adapter,
	schema string,
	req sdk.ListRequest,
	format string,
	stdout io.Writer,
) error {
	if schema == "" {
		return errors.New("report schema must not be empty")
	}
	list, _, err := sdk.ListAspect(ctxt, sdk.AspectSelector{
		ListRequest:    req,
		SchemaPrefix:   schema,
		IncludeContent: true, // the accession is in the report itself
	}, adapter, logger)
	if err != nil {
		return fmt.Errorf("cannot list the imported genomes: %w", err)
	}
	out := genomeListFrom(list, schema)
	if format == "json" || format == "yaml" {
		return printGenomeStructured(stdout, out, format)
	}
	printGenomeTable(stdout, out)
	return nil
}

// genomeListFrom turns an aspect list into rows. The API filters schemas by prefix, so only
// aspects with exactly this schema count, and only those that name the artifact (the entity)
// they belong to.
func genomeListFrom(list *api.ListResponseBody, schema string) genomeListOutput {
	out := genomeListOutput{Genomes: make([]genomeListRow, 0, len(list.Items))}
	for _, item := range list.Items {
		if item.Schema == nil || *item.Schema != schema || item.ID == nil || item.Entity == nil {
			continue
		}
		accession := genomeUnknownAccession
		var organism, assemblyName string
		if content, ok := item.Content.(map[string]any); ok {
			if s, ok := content["accession"].(string); ok && s != "" {
				accession = s
			}
			organism = reportString(content, "organism", "organism_name")
			assemblyName = reportString(content, "assembly_info", "assembly_name")
		}
		out.Genomes = append(out.Genomes, genomeListRow{
			Accession: accession, Organism: organism, AssemblyName: assemblyName,
			ReportAspect: *item.ID, Artifact: *item.Entity,
		})
	}
	if list.AtTime != nil {
		out.AtTime = *list.AtTime
	}
	if next := findNextAspectPage(list.Links); next != nil {
		out.nextLink = next
		if u, err := url.Parse(*next); err == nil {
			out.NextPage = u.Query().Get("page")
		}
	}
	return out
}

// printGenomeTable lays the list out like the other aspect-backed lists (job list, collection list,
// datafabric query): the records in a table of their own, inside a borderless table that also
// carries the time of the query and the reference to the next page.
func printGenomeTable(w io.Writer, out genomeListOutput) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"Accession", "Report aspect", "Artifact"})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(out.Genomes))
	for i, g := range out.Genomes {
		aspect, artifact := g.ReportAspect, g.Artifact
		// a short @n reference, or the full URN with --no-history
		rows[i] = table.Row{g.Accession, MakeHistory(&aspect), MakeHistory(&artifact)}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
	})

	p := []table.Row{}
	if out.AtTime != "" {
		p = append(p, table.Row{"At Time", safeDate(&out.AtTime, false)})
	}
	p = append(p, table.Row{"Genomes", tw2.Render()})
	p = addNextPageRow(out.nextLink, p)
	tw.AppendRows(p)

	fmt.Fprintf(w, "\n%s\n\n", tw.Render())
}

// reportString reads a string from the content of a report aspect, following a path of object keys.
func reportString(content map[string]any, path ...string) string {
	var cur any = content
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[key]
	}
	s, _ := cur.(string)
	return s
}
