package cmd

import (
	api "cayp/api_gateway/gen/http/order/client"
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/reinventingscience/ivcap-client/pkg"
	a "github.com/reinventingscience/ivcap-client/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

var (
	name string

	orderCmd = &cobra.Command{
		Use:     "order",
		Aliases: []string{"orders"},
		Short:   "Create and manage orders ",
	}

	listOrderCmd = &cobra.Command{
		Use:   "list",
		Short: "List existing orders",

		RunE: func(cmd *cobra.Command, args []string) error {
			req := &sdk.ListOrderRequest{Offset: 0, Limit: 50}
			if offset > 0 {
				req.Offset = offset
			}
			if limit > 0 {
				req.Limit = limit
			}

			switch format {
			case "json", "yaml":
				if res, err := sdk.ListOrdersRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					a.ReplyPrinter(res, format == "yaml")
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
		Use:     "read [flags] order-id",
		Aliases: []string{"get"},
		Short:   "Fetch details about a single order",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := args[0]
			req := &sdk.ReadOrderRequest{recordID}

			switch format {
			case "json", "yaml":
				if res, err := sdk.ReadOrderRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					a.ReplyPrinter(res, format == "yaml")
				} else {
					return err
				}
			default:
				if order, err := sdk.ReadOrder(context.Background(), req, CreateAdapter(true), logger); err == nil {
					printOrder(order, false)
				} else {
					return err
				}
			}
			return nil
		},
	}

	createOrderCmd = &cobra.Command{
		Use:   "create [flags] service-id [... paramName=value]",
		Short: "Create a new order",
		Long: `Create a new order for a service identified by it's id and add any 
potential paramters using the format 'paramName=value'. Please not that there
cannot be any spaces between the parameter name, the '=' and the value. If the value
contains spaces, put it into quotes which will not be removed by your shell.

An example:

  ivcap order create --name "test order" cayp:service:d939b74d-0070-59a4-a832-36c5c07e657d msg="Hello World"
	
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			serviceId := args[0]

			// fetch defined parameters to do some early verification
			service, err := sdk.ReadService(ctxt, &sdk.ReadServiceRequest{Id: serviceId}, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			paramSet := map[string]bool{}
			for _, p := range service.Parameters {
				paramSet[*p.Name] = true
			}
			params := make([]*api.ParameterT, len(args)-1)
			for i, ps := range args[1:] {
				pa := strings.SplitN(ps, "=", 2)
				if len(pa) != 2 {
					cobra.CheckErr(fmt.Sprintf("cannot parse parameter argument '%s'", ps))
				}
				name := pa[0]
				value := pa[1]
				if _, ok := paramSet[name]; !ok {
					cobra.CheckErr(fmt.Sprintf("parameter '%s' is not defined by the requested service", name))
				}
				params[i] = &api.ParameterT{Name: &name, Value: &value}
			}

			req := &api.CreateRequestBody{
				ServiceID:  serviceId,
				Parameters: params,
				AccountID:  GetAccountID(),
			}
			if name != "" {
				req.Name = &name
			}
			switch format {
			case "json", "yaml":
				if res, err := sdk.CreateOrderRaw(ctxt, req, CreateAdapter(true), logger); err == nil {
					a.ReplyPrinter(res, format == "yaml")
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
)

func init() {
	rootCmd.AddCommand(orderCmd)

	orderCmd.AddCommand(listOrderCmd)
	listOrderCmd.Flags().IntVar(&offset, "offset", -1, "record offset into returned list")
	listOrderCmd.Flags().IntVar(&limit, "limit", -1, "max number of records to be returned")
	listOrderCmd.Flags().StringVarP(&format, "output", "o", "short", "format to use for list (short, yaml, json)")

	orderCmd.AddCommand(readOrderCmd)
	readOrderCmd.Flags().StringVarP(&format, "output", "o", "short", "format to use for list (short, yaml, json)")

	orderCmd.AddCommand(createOrderCmd)
	createOrderCmd.Flags().StringVarP(&name, "name", "n", "", "Optional name/title attached to order")
	createOrderCmd.Flags().StringVarP(&format, "output", "o", "short", "format to use for list (short, yaml, json)")

}

func printOrdersTable(list *api.ListResponseBody, wide bool) {
	rows := make([]table.Row, len(list.Orders))
	for i, o := range list.Orders {
		rows[i] = table.Row{*o.ID, safeString(o.Name), safeString(o.Status), safeDate(o.OrderedAt), safeString(o.ServiceID)}
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Status", "Order At", "Service ID"})
	t.AppendRows(rows)
	t.Render()

}

func printOrder(order *api.ReadResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.SetStyle(table.StyleLight)
	tw2.SetColumnConfigs([]table.ColumnConfig{{Number: 1, Align: text.AlignRight}})
	tw2.Style().Options.SeparateColumns = false
	tw2.Style().Options.SeparateRows = false
	tw2.Style().Options.DrawBorder = true
	rows := make([]table.Row, len(order.Parameters))
	for i, p := range order.Parameters {
		rows[i] = table.Row{safeString(p.Name) + " =", safeString(p.Value)}
	}
	tw2.AppendRows(rows)

	tw3 := table.NewWriter()
	tw3.SetStyle(table.StyleLight)
	rows2 := make([]table.Row, len(order.Products))
	for i, p := range order.Products {
		rows2[i] = table.Row{safeString(p.ID), safeString(p.Name), safeString(p.MimeType)}
	}
	tw3.AppendRows(rows2)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 80},
	})

	tw.AppendRows([]table.Row{
		{"ID", *order.ID},
		{"Name", safeString(order.Name)},
		{"Status", safeString(order.Status)},
		{"Ordered at", safeDate(order.OrderedAt)},
		{"Service ID", safeString(order.Service.ID)},
		{"Account ID", safeString(order.Account.ID)},
		{"Parameters", tw2.Render()},
		{"Products", tw3.Render()},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}
