// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"fmt"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/aspect"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

func init() {
	rootCmd.AddCommand(datafabricCmd)

	datafabricCmd.AddCommand(aspectAddCmd)
	addFlags(aspectAddCmd, []Flag{Schema, InputFormat, Policy})
	aspectAddCmd.Flags().StringVarP(&aspectFile, "file", "f", "", "Path to file containing aspect content")

	datafabricCmd.AddCommand(aspectUpdateCmd)
	addFlags(aspectUpdateCmd, []Flag{Schema, InputFormat, Policy})
	aspectUpdateCmd.Flags().StringVarP(&aspectFile, "file", "f", "", "Path to file containing metdata")

	datafabricCmd.AddCommand(aspectGetCmd)
	aspectGetCmd.Flags().BoolVar(&aspectContentOnly, "content-only", false, "if set, only display the aspect's content part")

	datafabricCmd.AddCommand(datafabricQueryCmd)
	addFlags(datafabricQueryCmd, []Flag{SchemaPrefix, Entity})
	datafabricQueryCmd.Flags().BoolVarP(&aspectGetIfOne, "get-if-one", "g", false, "if only one found, get it immediately")
	datafabricQueryCmd.Flags().StringVarP(&aspectJsonFilter, "content-path", "c", "", "json path filter on aspect's content ('$.images[*] ? (@.size > 10000)')")
	datafabricQueryCmd.Flags().BoolVar(&aspectIncludeContent, "include-content", false, "if set, also include aspect's content in list")
	addListFlags(datafabricQueryCmd)

	datafabricCmd.AddCommand(aspectRetractCmd)
}

var (
	aspectFile string

	aspectJsonFilter     string
	aspectIncludeContent bool
	aspectGetIfOne       bool
	aspectContentOnly    bool
)

var (
	datafabricCmd = &cobra.Command{
		Use:     "datafabric",
		Aliases: []string{"df", "aspect", "as", "aspect"},
		Short:   "Query the datafabric and create and manage aspects within",
	}

	aspectAddCmd = &cobra.Command{
		Use:     "add entityURN [-s schemaName] -f -|aspect --format json|yaml [flags]",
		Short:   "Add aspect of a specific schema to an entity",
		Aliases: []string{"a", "+"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return addAspectUpdateCmd(true, cmd, args)
		},
	}

	aspectUpdateCmd = &cobra.Command{
		Use:     "update entityURN [-s schemaName] -f -|aspect --format json|yaml",
		Short:   "Update an aspect record for an entity and a specific schema",
		Aliases: []string{"a", "+"},
		Long:    `This command will only succeed if there is only one active record for the entity/schema pair`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return addAspectUpdateCmd(false, cmd, args)
		},
	}

	aspectGetCmd = &cobra.Command{
		Use:     "get aspectURN",
		Short:   "Get a specific aspect record",
		Aliases: []string{"g"},
		// Long:    `.....`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return getAspect(GetHistory(args[0]))
		},
	}

	aspectRetractCmd = &cobra.Command{
		Use:     "retract aspectURN [flags]",
		Short:   "Retract a specific aspect record",
		Aliases: []string{"r"},
		// Long:    `.....`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			aspectID := GetHistory(args[0])
			ctxt := context.Background()
			_, err = sdk.RetractAspect(ctxt, aspectID, CreateAdapter(true), logger)
			return
		},
	}

	datafabricQueryCmd = &cobra.Command{
		Use:     "query [-e entity] [-s schemaPrefix] [flags]",
		Short:   "Query the datafabric for any combination of entity, schema and time.",
		Aliases: []string{"q", "search", "s", "list", "l"},
		// Long:    `.....`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if entityURN == "" && schemaPrefix == "" && page == "" {
				cobra.CheckErr("Need at least one of '--schema', '--entity' or '--page'")
			}
			if entityURN != "" {
				entityURN = GetHistory(entityURN)
			}
			selector := sdk.AspectSelector{
				Entity:         entityURN,
				SchemaPrefix:   schemaPrefix,
				ListRequest:    *createListRequest(),
				IncludeContent: aspectIncludeContent,
			}

			if aspectJsonFilter != "" {
				selector.JsonFilter = &aspectJsonFilter
			}

			ctxt := context.Background()
			if list, res, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
				if aspectGetIfOne && len(list.Items) == 1 {
					return getAspect(*list.Items[0].ID)
				}
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					printAspectTable(list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}
)

func getAspect(aspectID string) error {
	ctxt := context.Background()
	if aspectContentOnly && outputFormat == "" {
		outputFormat = "yaml"
	}
	switch outputFormat {
	case "json", "yaml":
		if res, err := sdk.GetAspectRaw(ctxt, aspectID, CreateAdapter(true), logger); err == nil {
			if aspectContentOnly {
				if o, err := res.AsObject(); err == nil {
					if c, ok := o["content"]; ok {
						if s, err := a.ToString(c, outputFormat == "yaml"); err == nil {
							fmt.Printf("%s\n", s)
							return nil
						} else {
							return err
						}
					} else {
						return fmt.Errorf("aspect does not contain a 'Content' field")
					}
				} else {
					return err
				}
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		} else {
			return err
		}
	default:
		if res, err := sdk.GetAspect(ctxt, aspectID, CreateAdapter(true), logger); err == nil {
			printAspectDetail(res)
			return nil
		} else {
			return err
		}
	}
}

func addAspectUpdateCmd(isAdd bool, cmd *cobra.Command, args []string) (err error) {
	entity := args[0]
	pyld, err := payloadFromFile(aspectFile, inputFormat)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("While reading aspect file '%s' - %s", aspectFile, err))
	}

	aspect, err := pyld.AsObject()
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot parse aspect file '%s' - %s", aspectFile, err))
	}
	var schema string
	schema = schemaURN
	if schema == "" {
		if s, ok := aspect["$schema"]; ok {
			schema = fmt.Sprintf("%s", s)
		} else {
			cobra.CheckErr("Missing schema name")
		}
	}
	logger.Debug("add/update aspect", log.String("entity", entity), log.String("schema", schema), log.Reflect("pyld", aspect))
	ctxt := context.Background()
	res, err := sdk.AddUpdateAspect(ctxt, isAdd, entity, schema, policy, pyld.AsBytes(), CreateAdapter(true), logger)
	if err != nil {
		return err
	}
	if silent {
		if m, err := res.AsObject(); err == nil {
			fmt.Printf("%s\n", m["id"])
		} else {
			cobra.CheckErr(fmt.Sprintf("Parsing reply: %s", res.AsBytes()))
		}
	} else {
		return a.ReplyPrinter(res, outputFormat == "yaml")
	}
	return nil
}

func printAspectDetail(res *api.ReadResponseBody) {
	// ID *string `form:"id,omitempty" json:"id,omitempty" xml:"id,omitempty"`
	// Entity *string `form:"entity,omitempty" json:"entity,omitempty" xml:"entity,omitempty"`
	// Schema *string `form:"schema,omitempty" json:"schema,omitempty" xml:"schema,omitempty"`
	// Content any `form:"content,omitempty" json:"content,omitempty" xml:"content,omitempty"`
	// ContentType *string `json:"content-type,omitempty"`
	// ValidFrom *string `form:"valid-from,omitempty" json:"valid-from,omitempty" xml:"valid-from,omitempty"`
	// ValidTo *string `form:"valid-to,omitempty" json:"valid-to,omitempty" xml:"valid-to,omitempty"`
	// Asserter *string `form:"asserter,omitempty" json:"asserter,omitempty" xml:"asserter,omitempty"`
	// Retracter *string              `form:"retracter,omitempty"

	rows := []table.Row{
		{"ID", fmt.Sprintf("%s (%s)", *res.ID, MakeHistory(res.ID))},
		{"Entity", safeString(res.Entity)},
		{"Schema", safeString(res.Schema)},
		{"Asserter", safeString(res.Asserter)},
		{"ValidFrom", safeDate(res.ValidFrom, true)},
	}
	if res.ValidTo != nil {
		rows = append(rows,
			table.Row{"Retracter", safeString(res.Retracter)},
			table.Row{"ValidTo", safeDate(res.ValidTo, true)},
		)
	}
	if res.ContentType != nil && *res.ContentType == "application/json" {
		content, err := a.ToString(res.Content, false)
		if err != nil {
			fmt.Printf("ERROR: cannot print aspect content - %v\n", err)
			return
		}
		rows = append(rows, table.Row{"Content", content})
	} else {
		rows = append(rows,
			table.Row{"Content-Type", safeString(res.ContentType)},
			table.Row{"Content", ".... cannot print"},
		)
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		// {Number: 2, WidthMax: 80},
	})
	tw.AppendRows(rows)
	fmt.Printf("\n%s\n\n", tw.Render())
}

func printAspectTable(list *api.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"ID", "Entity", "Schema"})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(list.Items))
	for i, p := range list.Items {
		rows[i] = table.Row{MakeHistory(p.ID), safeString(p.Entity), safeString(p.Schema)}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		// {Number: 2, WidthMax: 80},
	})

	p := []table.Row{}
	if list.Entity != nil {
		p = append(p, table.Row{"Entity", *list.Entity})
	}
	if list.Schema != nil {
		p = append(p, table.Row{"Schema", *list.Schema})
	}
	if list.AtTime != nil {
		p = append(p, table.Row{"At Time", safeDate(list.AtTime, false)})
	}
	p = append(p, table.Row{"Records", tw2.Render()})
	p = addNextPageRow(findNextAspectPage(list.Links), p)
	tw.AppendRows(p)

	fmt.Printf("\n%s\n\n", tw.Render())
}

func findNextAspectPage(links []*api.LinkTResponseBody) *string {
	if links == nil {
		return nil
	}
	for _, l := range links {
		if l.Rel != nil && *l.Rel == "next" {
			return l.Href
		}
	}
	return nil
}
