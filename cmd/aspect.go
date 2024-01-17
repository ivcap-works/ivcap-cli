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

	"github.com/araddon/dateparse"
	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/aspect"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

func init() {
	rootCmd.AddCommand(aspectCmd)

	aspectCmd.AddCommand(aspectAddCmd)
	aspectAddCmd.Flags().StringVarP(&schemaURN, "schema", "s", "", "URN/UUID of schema")
	aspectAddCmd.Flags().StringVarP(&aspectFile, "file", "f", "", "Path to file containing metdata")
	aspectAddCmd.Flags().StringVarP(&inputFormat, "format", "", "json", "Format of service description file [json, yaml]")
	aspectAddCmd.Flags().StringVarP(&policy, "policy", "p", "", "Policy controlling access")

	aspectCmd.AddCommand(aspectUpdateCmd)
	aspectUpdateCmd.Flags().StringVarP(&schemaURN, "schema", "s", "", "URN/UUID of schema")
	aspectUpdateCmd.Flags().StringVarP(&aspectFile, "file", "f", "", "Path to file containing metdata")
	aspectUpdateCmd.Flags().StringVarP(&inputFormat, "format", "", "json", "Format of service description file [json, yaml]")
	aspectUpdateCmd.Flags().StringVarP(&policy, "policy", "p", "", "Policy controlling access")

	aspectCmd.AddCommand(aspectGetCmd)

	aspectCmd.AddCommand(aspectQueryCmd)
	aspectQueryCmd.Flags().StringVarP(&schemaPrefix, "schema", "s", "", "URN/UUID prefix of schema")
	aspectQueryCmd.Flags().StringVarP(&entityURN, "entity", "e", "", "URN/UUID of entity")
	aspectQueryCmd.Flags().StringVarP(&aspectJsonFilter, "json-path", "j", "", "json path filter on aspect ('$.images[*] ? (@.size > 10000)')")
	aspectQueryCmd.Flags().StringVarP(&aspectFilter, "filter", "f", "", "simple filter on aspect ('FirstName ~= 'Scott'')")
	aspectQueryCmd.Flags().StringVarP(&atTime, "time-at", "t", "", "Timestamp for which to request information [now]")
	aspectQueryCmd.Flags().IntVar(&limit, "limit", 10, "max number of records to be returned")
	aspectQueryCmd.Flags().StringVarP(&page, "page", "p", "", "query page token, for example to get next page")

	aspectCmd.AddCommand(aspectRetractCmd)
}

var (
	aspectFile string

// schemaURN        string
// schemaPrefix     string
// entityURN        string
// aspectJsonFilter string
// aspectFilter     string
// atTime           string
// page             string
)

var (
	aspectCmd = &cobra.Command{
		Use:     "aspect",
		Aliases: []string{"as", "aspect"},
		Short:   "Add/get/retract/query aspects",
	}

	aspectAddCmd = &cobra.Command{
		Use:     "add [flags] entity [-s schemaName] -f -|aspect --format json|yaml",
		Short:   "Add aspect of a specific schema to an entity",
		Aliases: []string{"a", "+"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return addAspectUpdateCmd(true, cmd, args)
		},
	}

	aspectUpdateCmd = &cobra.Command{
		Use:     "update entity [-s schemaName] -f -|aspect --format json|yaml",
		Short:   "Update an aspect record for an entity and a specific schema",
		Aliases: []string{"a", "+"},
		Long:    `This command will only succeed if there is only one active record for the entity/schema pair`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return addAspectUpdateCmd(false, cmd, args)
		},
	}

	aspectGetCmd = &cobra.Command{
		Use:     "get aspect-id",
		Short:   "Get a specifric aspect record",
		Aliases: []string{"g"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			aspectID := GetHistory(args[0])
			ctxt := context.Background()
			res, err := sdk.GetAspect(ctxt, aspectID, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}

	aspectRetractCmd = &cobra.Command{
		Use:     "retract [flags] aspect-id",
		Short:   "Retract a specific aspect record",
		Aliases: []string{"r"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			aspectID := GetHistory(args[0])
			ctxt := context.Background()
			_, err = sdk.RetractAspect(ctxt, aspectID, CreateAdapter(true), logger)
			return
		},
	}

	aspectQueryCmd = &cobra.Command{
		Use:     "query [-e entity] [-s schemaPrefix] [-t time-at]",
		Short:   "Query the aspect store for any combination of entity, schema and time.",
		Aliases: []string{"q", "search", "s", "list", "l"},
		Long:    `.....`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if entityURN == "" && schemaPrefix == "" && page == "" {
				cobra.CheckErr("Need at least one of '--schema', '--entity' or '--page'")
			}
			if entityURN != "" {
				entityURN = GetHistory(entityURN)
			}
			selector := sdk.AspectSelector{
				Entity:       entityURN,
				SchemaPrefix: schemaPrefix,
				Page:         GetHistory(page),
				Limit:        limit,
			}

			if aspectFilter != "" {
				selector.SimpleFilter = &aspectFilter
			}
			if aspectJsonFilter != "" {
				selector.JsonFilter = &aspectJsonFilter
			}
			if atTime != "" {
				t, err := dateparse.ParseLocal(atTime)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("Can't parse '%s' into a date - %s", atTime, err))
				}
				selector.Timestamp = &t
			}

			ctxt := context.Background()
			if list, res, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
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
			fmt.Printf("%s\n", m["record-id"])
		} else {
			cobra.CheckErr(fmt.Sprintf("Parsing reply: %s", res.AsBytes()))
		}
	} else {
		return a.ReplyPrinter(res, outputFormat == "yaml")
	}
	return nil
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
