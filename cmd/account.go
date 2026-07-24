// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

var accountName string

func init() {
	rootCmd.AddCommand(accountCmd)

	accountCmd.AddCommand(listAccountCmd)
	addListFlags(listAccountCmd)

	accountCmd.AddCommand(readAccountCmd)

	accountCmd.AddCommand(createAccountCmd)
	createAccountCmd.Flags().StringVarP(&accountName, "name", "n", "", "Display name for the new org account")
}

var (
	accountCmd = &cobra.Command{
		Use:     "account",
		Aliases: []string{"a", "accounts"},
		Short:   "Manage accounts you belong to",
	}

	listAccountCmd = &cobra.Command{
		Use:   "list",
		Short: "List accounts you can access",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := createListRequest()
			res, err := sdk.ListAccountsRaw(context.Background(), req, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json":
				return a.ReplyPrinter(res, false)
			case "yaml":
				return a.ReplyPrinter(res, true)
			default:
				var list accountsapi.ListAccountsResult
				if err = res.AsType(&list); err != nil {
					return err
				}
				printAccountTable(list.Accounts)
			}
			return nil
		},
	}

	readAccountCmd = &cobra.Command{
		Use:     "get [flags] account_id",
		Aliases: []string{"read"},
		Short:   "Fetch details about a single account",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			res, err := sdk.ReadAccountRaw(context.Background(), id, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json", "yaml":
				return a.ReplyPrinter(res, outputFormat == "yaml")
			default:
				var acc accountsapi.Account
				if err = res.AsType(&acc); err != nil {
					return err
				}
				printAccount(&acc)
			}
			return nil
		},
	}

	createAccountCmd = &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a new organisation account",
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountName == "" {
				return fmt.Errorf("please provide a name via --name")
			}
			res, err := sdk.CreateAccountRaw(context.Background(), accountName, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}
)

// truncString shortens a display name to fit the name column.
func truncString(in string) string {
	if len(in) > MAX_NAME_COL_LEN {
		return in[0:MAX_NAME_COL_LEN-3] + "..."
	}
	return in
}

func printAccountTable(accounts []accountsapi.Account) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Kind", "Name"})
	rows := make([]table.Row, len(accounts))
	for i, acc := range accounts {
		id := acc.Id
		rows[i] = table.Row{MakeHistory(&id), acc.Kind, truncString(acc.Name)}
	}
	t.AppendRows(rows)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 3, WidthMaxEnforcer: text.WrapSoft},
	})
	t.Style().Options.SeparateRows = true
	t.Render()
}

func printAccount(acc *accountsapi.Account) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	id := acc.Id
	tw.AppendRows([]table.Row{
		{"Name", acc.Name},
		{"ID", fmt.Sprintf("%s (%s)", acc.Id, MakeHistory(&id))},
		{"Kind", acc.Kind},
	})
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}
