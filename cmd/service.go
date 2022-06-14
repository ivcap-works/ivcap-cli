package cmd

import (
	api "cayp/api_gateway/gen/http/service/client"
	"context"
	"fmt"
	"os"

	sdk "github.com/reinventingscience/ivcap-client/pkg"
	a "github.com/reinventingscience/ivcap-client/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

var (
	serviceCmd = &cobra.Command{
		Use:     "service",
		Aliases: []string{"s", "services"},
		Short:   "Create and manage services ",
	}

	listServiceCmd = &cobra.Command{
		Use:   "list",
		Short: "List existing service",

		RunE: func(cmd *cobra.Command, args []string) error {
			req := &sdk.ListServiceRequest{Offset: 0, Limit: 50}
			if offset > 0 {
				req.Offset = offset
			}
			if limit > 0 {
				req.Limit = limit
			}
			if res, err := sdk.ListServicesRaw(context.Background(), req, CreateAdapter(false), logger); err == nil {
				switch format {
				case "json":
					a.ReplyPrinter(res, false)
				case "yaml":
					a.ReplyPrinter(res, true)
				default:
					var list api.ListResponseBody
					res.AsType(&list)
					printServiceTable(&list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	readServiceCmd = &cobra.Command{
		Use:     "read [flags] service_id",
		Aliases: []string{"get"},
		Short:   "Fetch details about a single service",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := args[0]
			req := &sdk.ReadServiceRequest{Id: recordID}

			switch format {
			case "json", "yaml":
				if res, err := sdk.ReadServiceRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					a.ReplyPrinter(res, format == "yaml")
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

	// createServiceCmd = &cobra.Command{
	// 	Use:   "create",
	// 	Short: "Create a new service",

	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		fmt.Printf("service called %v - %v\n", recordID, args)
	// 	},
	// }
)

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.AddCommand(listServiceCmd)
	listServiceCmd.Flags().IntVar(&offset, "offset", -1, "record offset into returned list")
	listServiceCmd.Flags().IntVar(&limit, "limit", -1, "max number of records to be returned")
	listServiceCmd.Flags().StringVarP(&format, "output", "o", "short", "format to use for list (short, yaml, json)")

	serviceCmd.AddCommand(readServiceCmd)
	readServiceCmd.Flags().StringVarP(&recordID, "service-id", "i", "", "ID of service to retrieve")

	// serviceCmd.AddCommand(createCmd)
	// createCmd.Flags().StringVarP(&recordID, "service-id", "i", "", "ID of service to manage")
}

func printServiceTable(list *api.ListResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Provider"})
	rows := make([]table.Row, len(list.Services))
	for i, o := range list.Services {
		rows[i] = table.Row{*o.ID, safeTruncString(o.Name), safeString(o.Provider.ID)}
	}
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
	tw2.AppendHeader(table.Row{"Name", "Description", "Type", "Default"})
	rows := make([]table.Row, len(service.Parameters))
	for i, p := range service.Parameters {
		rows[i] = table.Row{safeString(p.Name), safeString(p.Description), safeString(p.Type), safeString(p.Default)}
	}
	tw2.AppendRows(rows)

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
		{"ID", *service.ID},
		{"Name", safeString(service.Name)},
		{"Description", safeString(service.Description)},
		{"Status", safeString(service.Status)},
		{"Provider ID", safeString(service.Provider.ID)},
		{"Account ID", safeString(service.Account.ID)},
		{"Parameters", tw2.Render()},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}
