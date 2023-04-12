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
	"time"

	"github.com/araddon/dateparse"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	sdk "github.com/reinventingscience/ivcap-client/pkg"
	a "github.com/reinventingscience/ivcap-client/pkg/adapter"
	api "github.com/reinventingscience/ivcap-core-api/http/metadata"

	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

func init() {
	rootCmd.AddCommand(metaCmd)

	metaCmd.AddCommand(metaAddCmd)
	metaAddCmd.Flags().StringVarP(&schemaURN, "schema", "s", "", "URN/UUID of schema")
	metaAddCmd.Flags().StringVarP(&metaFile, "file", "f", "", "Path to file containing metdata")
	metaAddCmd.Flags().StringVarP(&inputFormat, "format", "", "json", "Format of service description file [json, yaml]")

	metaCmd.AddCommand(metaGetCmd)

	metaCmd.AddCommand(metaQueryCmd)
	metaQueryCmd.Flags().StringVarP(&schemaPrefix, "schema", "s", "", "URN/UUID prefix of schema")
	metaQueryCmd.Flags().StringVarP(&entityURN, "entity", "e", "", "URN/UUID of entity")
	metaQueryCmd.Flags().StringVarP(&atTime, "time-at", "t", "", "Timestamp for which to request information [now]")

	metaCmd.AddCommand(metaRevokeCmd)
}

var schemaURN string
var schemaPrefix string
var entityURN string
var atTime string

var (
	metaCmd = &cobra.Command{
		Use:     "metadata",
		Aliases: []string{"m", "meta"},
		Short:   "Add/get/revoke/query metadata",
	}

	metaAddCmd = &cobra.Command{
		Use:     "add [flags] entity [-s schemaName] -f -|meta --format json|yaml",
		Short:   "Add metadata of a specific schema to an entiry",
		Aliases: []string{"a", "+"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			entity := args[0]
			pyld, err := payloadFromFile(metaFile, inputFormat)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("While reading metadata file '%s' - %s", metaFile, err))
			}

			meta, err := pyld.AsObject()
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Cannot parse meta file '%s' - %s", metaFile, err))
			}
			var schema string
			schema = schemaURN
			if schema == "" {
				if s, ok := meta["$schema"]; ok {
					schema = fmt.Sprintf("%s", s)
				} else {
					cobra.CheckErr("Missing schema name")
				}
			}
			logger.Debug("add meta", log.String("entity", entity), log.String("schema", schema), log.Reflect("pyld", meta))
			ctxt := context.Background()
			if res, err := sdk.AddMetadata(ctxt, entity, schema, pyld.AsBytes(), CreateAdapter(true), logger); err == nil {
				if silent {
					if m, err := res.AsObject(); err == nil {
						fmt.Printf("%s\n", m["record-id"])
					} else {
						cobra.CheckErr(fmt.Sprintf("Parsing reply: %s", res.AsBytes()))
					}
				} else {
					a.ReplyPrinter(res, outputFormat == "yaml")
				}
			} else {
				return err
			}
			return nil
		},
	}

	metaGetCmd = &cobra.Command{
		Use:     "get recordID",
		Short:   "Get the metadata record",
		Aliases: []string{"g"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			recordID := args[0]
			ctxt := context.Background()
			if res, err := sdk.GetMetadata(ctxt, GetHistory(recordID), CreateAdapter(true), logger); err == nil {
				a.ReplyPrinter(res, outputFormat == "yaml")
				return nil
			} else {
				return err
			}
		},
	}

	metaRevokeCmd = &cobra.Command{
		Use:     "revoke [flags] record-id",
		Short:   "Revoke a specific metadata record",
		Aliases: []string{"r"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			recordID := args[0]
			ctxt := context.Background()
			_, err = sdk.RevokeMetadata(ctxt, recordID, CreateAdapter(true), logger)
			return
		},
	}

	metaQueryCmd = &cobra.Command{
		Use:     "query [-e entity] [-s schemaPrefix] [-t time-at]",
		Short:   "Query the metadata store for any combination of entity, schema and time.",
		Aliases: []string{"q", "search", "s", "list", "l"},
		Long:    `.....`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if entityURN == "" && schemaPrefix == "" {
				cobra.CheckErr("Need at least one of '--schema' or '--entity'")
			}
			if entityURN != "" {
				entityURN = GetHistory(entityURN)
			}
			var ts *time.Time
			if atTime != "" {
				t, err := dateparse.ParseLocal(atTime)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("Can't parse '%s' into a date - %s", atTime, err))
				}
				ts = &t
			}
			ctxt := context.Background()
			if list, res, err := sdk.ListMetadata(ctxt, entityURN, schemaPrefix, ts, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					a.ReplyPrinter(res, false)
				case "yaml":
					a.ReplyPrinter(res, true)
				default:
					printMetadataTable(list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}
)

func printMetadataTable(list *api.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"ID", "Entity", "Schema"})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(list.Records))
	for i, p := range list.Records {
		rows[i] = table.Row{MakeHistory(p.RecordID), safeString(p.Entity), safeString(p.Schema)}
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
	if list.EntityID != nil {
		p = append(p, table.Row{"Entity", *list.EntityID})
	}
	if list.Schema != nil {
		p = append(p, table.Row{"Schema", *list.Schema})
	}
	if list.AtTime != nil {
		p = append(p, table.Row{"At Time", safeDate(list.AtTime, false)})
	}
	p = append(p, table.Row{"Records", tw2.Render()})

	tw.AppendRows(p)
	fmt.Printf("\n%s\n\n", tw.Render())
}
