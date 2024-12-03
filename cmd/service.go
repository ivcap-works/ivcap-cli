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
	"os"
	"strings"

	api "github.com/ivcap-works/ivcap-core-api/http/service"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serviceCmd)

	// LIST
	serviceCmd.AddCommand(listServiceCmd)
	addListFlags(listServiceCmd)

	// READ
	serviceCmd.AddCommand(readServiceCmd)

	// CREATE
	serviceCmd.AddCommand(createServiceCmd)
	addFileFlag(createServiceCmd, "Path to service description file")
	// createServiceCmd.Flags().StringVarP(&serviceFile, "file", "f", "", "Path to service description file")
	createServiceCmd.Flags().StringVar(&inputFormat, "format", "", "Format of service description file [json, yaml]")

	serviceCmd.AddCommand(updateServiceCmd)
	addFlags(updateServiceCmd, []Flag{InputFormat})
	updateServiceCmd.Flags().BoolVarP(&createAnyway, "create", "", false, "Create service record if it doesn't exist")
	addFileFlag(updateServiceCmd, "Path to service description file")
}

var createAnyway bool
var inputFormat string

var (
	serviceCmd = &cobra.Command{
		Use:     "service",
		Aliases: []string{"s", "services"},
		Short:   "Create and manage services",
	}

	listServiceCmd = &cobra.Command{
		Use:   "list",
		Short: "List existing service",

		RunE: func(cmd *cobra.Command, args []string) error {
			req := createListRequest()
			if res, err := sdk.ListServicesRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var list api.ListResponseBody
					if err = res.AsType(&list); err != nil {
						return err
					}
					printServiceTable(&list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	readServiceCmd = &cobra.Command{
		Use:     "get [flags] service_id",
		Aliases: []string{"read"},
		Short:   "Fetch details about a single service",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := GetHistory(args[0])
			req := &sdk.ReadServiceRequest{Id: GetHistory(recordID)}

			switch outputFormat {
			case "json", "yaml":
				if res, err := sdk.ReadServiceRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					return a.ReplyPrinter(res, outputFormat == "yaml")
				} else {
					return err
				}
			default:
				if service, err := sdk.ReadService(context.Background(), req, CreateAdapter(true), logger); err == nil {
					printService(service, false)
				} else {
					return err
				}
			}
			return nil
		},
	}

	createServiceCmd = &cobra.Command{
		Use:   "create [flags] -f service-file|-",
		Short: "Create a new service",
		Long: `Define a new service to available on the platform. The service is
described in a service definition file. If the service definition is provided
through 'stdin' use '-' as the file name and also include the --format flag`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()

			pyld, err := payloadFromFile(fileName, inputFormat)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("While reading service file '%s' - %s", fileName, err))
			}
			var req api.CreateServiceRequestBody
			if err = pyld.AsType(&req); err != nil {
				return
			}
			res, err := sdk.CreateServiceRaw(ctxt, &req, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}

	updateServiceCmd = &cobra.Command{
		Use:   "update [flags] service-id -f service-file|-",
		Short: "Update an existing service",
		Long: `Update an existing service description or create it if it does not exist
AND the --create flag is set. If the service definition is provided
through 'stdin' use '-' as the file name and also include the --format flag`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			serviceID := GetHistory(args[0])
			// serviceFile := args[1]

			isYaml := inputFormat == "yaml" || strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml")
			var pyld a.Payload
			if fileName != "-" {
				pyld, err = a.LoadPayloadFromFile(fileName, isYaml)
			} else {
				pyld, err = a.LoadPayloadFromStdin(isYaml)
			}
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("While reading service file '%s' - %s", fileName, err))
			}

			var req api.UpdateRequestBody
			if err = pyld.AsType(&req); err != nil {
				return
			}
			res, err := sdk.UpdateServiceRaw(ctxt, serviceID, createAnyway, &req, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}
)

func printServiceTable(list *api.ListResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Account"})
	rows := make([]table.Row, len(list.Items))
	for i, o := range list.Items {
		rows[i] = table.Row{MakeHistory(o.ID), safeTruncString(o.Name), safeString(o.Account)}
	}
	rows = addNextPageRow(findNextServicePage(list.Links), rows)
	t.AppendRows(rows)
	t.Render()
}

func printService(service *api.ReadResponseBody, wide bool) {

	// Name        *string                      `form:"name,omitempty" json:"name,omitempty" xml:"name,omitempty"`
	// Label       *string                      `form:"label,omitempty" json:"label,omitempty" xml:"label,omitempty"`
	// Type        *string                      `form:"type,omitempty" json:"type,omitempty" xml:"type,omitempty"`
	// Description *string                      `form:"description,omitempty" json:"description,omitempty" xml:"description,omitempty"`
	// Unit        *string                      `form:"unit,omitempty" json:"unit,omitempty" xml:"unit,omitempty"`
	// Constant    *bool                        `form:"constant,omitempty" json:"constant,omitempty" xml:"constant,omitempty"`
	// Optional    *bool                        `form:"optional,omitempty" json:"optional,omitempty" xml:"optional,omitempty"`
	// Default     *string                      `form:"default,omitempty" json:"default,omitempty" xml:"default,omitempty"`
	// Options     []*ParameterOptTResponseBody `form:"options,omitempty" json:"options,omitempty" xml:"options,omitempty"`
	tw2 := table.NewWriter()
	tw2.SetStyle(table.StyleLight)
	tw2.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: MAX_NAME_COL_LEN},
	})
	tw2.Style().Options.SeparateRows = true
	tw2.AppendHeader(table.Row{"Name", "Description", "Type", "Default", "Optional"})
	rows := make([]table.Row, len(service.Parameters))
	for i, p := range service.Parameters {
		ptype := getPType(p)
		var optional bool
		if p.Optional != nil {
			optional = *p.Optional
		}
		rows[i] = table.Row{safeString(p.Name), safeString(p.Description), ptype, safeString(p.Default), optional}
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
	tw.AppendRows([]table.Row{
		{"ID", fmt.Sprintf("%s (%s)", *service.ID, MakeHistory(service.ID))},
		{"Name", safeString(service.Name)},
		{"Description", safeString(service.Description)},
		{"Status", safeString(service.Status)},
		{"Account ID", safeString(service.Account)},
		{"Parameters", tw2.Render()},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func getPType(p *api.ParameterDefTResponseBody) string {
	if p == nil {
		return "???"
	}
	if p.Options == nil {
		// normal type
		return *p.Type
	}
	oa := make([]string, len(p.Options))
	for i, el := range p.Options {
		oa[i] = *el.Value
	}
	return fmt.Sprintf("[%s]", strings.Join(oa, ","))
}

func GetServiceNameForId(serviceID *string) string {
	if serviceID == nil {
		return "???"
	}
	req := &sdk.ReadServiceRequest{
		Id: *serviceID,
	}
	if resp, err := sdk.ReadService(context.Background(), req, CreateAdapter(true), logger); err == nil {
		return *resp.Name
	} else {
		return *serviceID
	}
}

func findNextServicePage(links []*api.LinkTResponseBody) *string {
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
