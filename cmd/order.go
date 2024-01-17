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
	"time"

	meta "github.com/ivcap-works/ivcap-core-api/http/metadata"
	api "github.com/ivcap-works/ivcap-core-api/http/order"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(orderCmd)

	// LIST
	orderCmd.AddCommand(listOrderCmd)
	listOrderCmd.Flags().IntVar(&limit, "limit", -1, "max number of records to be returned")
	listOrderCmd.Flags().StringVarP(&page, "page", "p", "", "page cursor")
	listOrderCmd.Flags().StringVarP(&outputFormat, "output", "o", "short", "format to use for list (short, yaml, json)")

	// READ
	orderCmd.AddCommand(readOrderCmd)
	readOrderCmd.Flags().StringVarP(&outputFormat, "output", "o", "short", "format to use for list (short, yaml, json)")

	// CREATE
	orderCmd.AddCommand(createOrderCmd)
	createOrderCmd.Flags().StringVarP(&name, "name", "n", "", "Optional name/title attached to order")
	createOrderCmd.Flags().StringVarP(&outputFormat, "output", "o", "short", "format to use for list (short, yaml, json)")
	createOrderCmd.Flags().StringVar(&accountID, "account-id", "", "override the account ID to use for the order")
	createOrderCmd.Flags().BoolVar(&skipParameterCheck, "skip-parameter-check", false, "fskip checking order paramters first ONLY USE FOR TESTING")

	// Logs
	orderCmd.AddCommand(downloadLogCmd)
	downloadLogCmd.Flags().StringVar(&downloadLogFrom, "from", "", "from time string in format YYYY-MM-DDTHH:MI:SS")
	downloadLogCmd.Flags().StringVar(&downloadLogTo, "to", "", "from time string in format YYYY-MM-DDTHH:MI:SS")

	// Top
	orderCmd.AddCommand(topCmd)
}

var (
	name                           string
	accountID                      string
	skipParameterCheck             bool
	downloadLogFrom, downloadLogTo string

	orderCmd = &cobra.Command{
		Use:     "order",
		Aliases: []string{"o", "orders"},
		Short:   "Create and manage orders ",
	}

	listOrderCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List existing orders",

		RunE: func(cmd *cobra.Command, args []string) error {
			req := &sdk.ListOrderRequest{Offset: 0, Limit: 50}
			if limit > 0 {
				req.Limit = limit
			}
			if page != "" {
				p := GetHistory(page)
				req.Page = &p
			}

			switch outputFormat {
			case "json", "yaml":
				if res, err := sdk.ListOrdersRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					return a.ReplyPrinter(res, outputFormat == "yaml")
				} else {
					return err
				}
			default:
				if list, err := sdk.ListOrders(context.Background(), req, CreateAdapter(true), logger); err == nil {
					printOrdersTable(list, false)
				} else {
					return err
				}
			}
			return nil
		},
	}

	readOrderCmd = &cobra.Command{
		Use:     "get [flags] order-id",
		Aliases: []string{"read", "r", "g"},
		Short:   "Fetch details about a single order",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := GetHistory(args[0])
			req := &sdk.ReadOrderRequest{Id: recordID}
			adapter := CreateAdapter(true)

			switch outputFormat {
			case "json", "yaml":
				if res, err := sdk.ReadOrderRaw(context.Background(), req, adapter, logger); err == nil {
					return a.ReplyPrinter(res, outputFormat == "yaml")
				} else {
					return err
				}
			default:
				if order, err := sdk.ReadOrder(context.Background(), req, adapter, logger); err == nil {
					selector := sdk.MetadataSelector{Entity: recordID}
					if meta, _, err := sdk.ListMetadata(context.Background(), selector, adapter, logger); err == nil {
						printOrder(order, meta, false)
					} else {
						return err
					}
				} else {
					return err
				}
			}
			return nil
		},
	}

	createOrderCmd = &cobra.Command{
		Use:     "create [flags] service-id [... paramName=value]",
		Aliases: []string{"c"},
		Short:   "Create a new order",
		Long: `Create a new order for a service identified by it's id and add any
potential paramters using the format 'paramName=value'. Please not that there
cannot be any spaces between the parameter name, the '=' and the value. If the value
contains spaces, put it into quotes which will not be removed by your shell.

An example:

  ivcap order create --name "test order" ivcap:service:d939b74d-0070-59a4-a832-36c5c07e657d msg="Hello World"

`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			serviceId := GetHistory(args[0])

			var paramSet = map[string]bool{}
			if !skipParameterCheck {
				// fetch defined parameters to do some early verification
				service, err := sdk.ReadService(ctxt, &sdk.ReadServiceRequest{Id: serviceId}, CreateAdapter(true), logger)
				if err != nil {
					return err
				}
				for _, p := range service.Parameters {
					paramSet[*p.Name] = true
				}
			}
			params := make([]*api.ParameterT, len(args)-1)
			for i, ps := range args[1:] {
				pa := strings.SplitN(ps, "=", 2)
				if len(pa) != 2 {
					cobra.CheckErr(fmt.Sprintf("cannot parse parameter argument '%s'", ps))
				}
				name := pa[0]
				value := pa[1]
				if !skipParameterCheck {
					if _, ok := paramSet[name]; !ok {
						cobra.CheckErr(fmt.Sprintf("parameter '%s' is not defined by the requested service", name))
					}
				}
				params[i] = &api.ParameterT{Name: &name, Value: &value}
			}

			req := &api.CreateRequestBody{
				Service:    serviceId,
				Parameters: params,
			}
			if name != "" {
				req.Name = &name
			}
			switch outputFormat {
			case "json", "yaml":
				if res, err := sdk.CreateOrderRaw(ctxt, req, CreateAdapter(true), logger); err == nil {
					return a.ReplyPrinter(res, outputFormat == "yaml")
				} else {
					return err
				}
			default:
				if res, err := sdk.CreateOrder(ctxt, req, CreateAdapter(true), logger); err == nil {
					fmt.Printf("Order '%s' with status '%s' submitted.\n", *res.ID, *res.Status)
				} else {
					return err
				}
			}
			return nil
		},
	}

	downloadLogCmd = &cobra.Command{
		Use:   "logs [flags] order-id",
		Short: "Download order logs for specific order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := GetHistory(args[0])
			req := &sdk.LogsRequestBody{
				OrderID: recordID,
			}
			if downloadLogFrom != "" {
				t, err := time.Parse(time.RFC3339, downloadLogFrom)
				if err != nil {
					return fmt.Errorf("invalid from parameter format: %s", downloadLogFrom)
				}
				tm := t.Unix()
				req.From = tm
			}
			if downloadLogTo != "" {
				t, err := time.Parse(time.RFC3339, downloadLogTo)
				if err != nil {
					return fmt.Errorf("invalid to parameter format: %s", downloadLogTo)
				}
				tm := t.Unix()
				req.To = tm
			}

			adapter := CreateAdapter(true)
			return sdk.DownloadOrderLog(context.Background(), req, adapter, logger)
		},
	}

	topCmd = &cobra.Command{
		Use:   "top [flags] order-id",
		Short: "check container resources for specific order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := GetHistory(args[0])

			adapter := CreateAdapter(true)
			ctx := context.Background()
			res, err := sdk.TopOrderRaw(ctx, recordID, adapter, logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json":
				return a.ReplyPrinter(res, outputFormat == "yaml")
			default:
				return a.ReplyPrinter(res, true)
			}
		},
	}
)

func printOrdersTable(list *api.ListResponseBody, wide bool) {
	srv2name := make(map[string]string)
	rows := make([]table.Row, len(list.Items))
	for i, o := range list.Items {
		var serviceName string
		if o.Service != nil {
			var ok bool
			if serviceName, ok = srv2name[*o.Service]; !ok {
				serviceName = GetServiceNameForId(o.Service)
				srv2name[*o.Service] = serviceName
			}
		}
		rows[i] = table.Row{MakeHistory(o.ID), safeString(o.Name), safeString(o.Status),
			safeDate(o.OrderedAt, true), serviceName}
	}
	rows = addNextPageRow(findNextOrderPage(list.Links), rows)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Status", "Order At", "Service ID"})
	t.AppendRows(rows)
	t.Render()
}

func printOrder(order *api.ReadResponseBody, meta *meta.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.SetStyle(table.StyleLight)
	tw2.SetColumnConfigs([]table.ColumnConfig{{Number: 1, Align: text.AlignRight}})
	tw2.Style().Options.SeparateColumns = false
	tw2.Style().Options.SeparateRows = false
	tw2.Style().Options.DrawBorder = true
	rows := make([]table.Row, len(order.Parameters))
	for i, p := range order.Parameters {
		rows[i] = table.Row{safeString(p.Name) + " =", MakeMaybeHistory(p.Value)}
	}
	tw2.AppendRows(rows)

	tw3 := table.NewWriter()
	tw3.SetStyle(table.StyleLight)
	if order.Products != nil {
		rows2 := make([]table.Row, len(order.Products.Items))
		for i, p := range order.Products.Items {
			rows2[i] = table.Row{MakeHistory(p.ID), safeString(p.Name), safeString(p.MimeType)}
		}
		rows2 = addNextPageRow(findNextOrderPage(order.Products.Links), rows2)
		tw3.AppendRows(rows2)
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

	tw4 := table.NewWriter()
	tw4.SetStyle(table.StyleLight)
	if meta != nil {
		rows2 := make([]table.Row, len(meta.Items))
		for i, p := range meta.Items {
			rows2[i] = table.Row{MakeHistory(p.ID), safeString(p.Schema)}
		}
		tw4.AppendRows(rows2)
	}

	tw.AppendRows([]table.Row{
		{"ID", *order.ID},
		{"Name", safeString(order.Name)},
		{"Status", safeString(order.Status)},
		{"Ordered", safeDate(order.OrderedAt, false)},
		{"Service", fmt.Sprintf("%s (%s)", GetServiceNameForId(order.Service), MakeHistory(order.Service))},
		{"Account ID", safeString(order.Account)},
		{"Parameters", tw2.Render()},
		{"Products", tw3.Render()},
		{"Metadata", tw4.Render()},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func findNextOrderPage(links []*api.LinkTResponseBody) *string {
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
