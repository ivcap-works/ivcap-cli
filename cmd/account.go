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
	"strings"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

var (
	accountName       string
	acctPrincipalUser string
	acctCapabilities  []string
	acctInviteEmail   string
)

func init() {
	rootCmd.AddCommand(accountCmd)

	accountCmd.AddCommand(listAccountCmd)
	addListFlags(listAccountCmd)

	accountCmd.AddCommand(readAccountCmd)

	accountCmd.AddCommand(createAccountCmd)
	createAccountCmd.Flags().StringVarP(&accountName, "name", "n", "", "Display name for the new org account")

	accountCmd.AddCommand(membersAccountCmd)

	accountCmd.AddCommand(grantAccountCmd)
	grantAccountCmd.Flags().StringVar(&acctPrincipalUser, "user", "", "User URN to grant capabilities to")
	grantAccountCmd.Flags().StringSliceVarP(&acctCapabilities, "capability", "c", nil,
		"Capability to grant (repeatable). Run 'ivcap capabilities --kind account' to list valid values")

	accountCmd.AddCommand(revokeAccountCapabilityCmd)
	revokeAccountCapabilityCmd.Flags().StringVar(&acctPrincipalUser, "user", "", "User URN to revoke from")
	revokeAccountCapabilityCmd.Flags().StringSliceVarP(&acctCapabilities, "capability", "c", nil,
		"Capability to revoke (repeatable)")

	accountCmd.AddCommand(removeAccountMemberCmd)
	removeAccountMemberCmd.Flags().StringVar(&acctPrincipalUser, "user", "", "User URN to remove from the account")

	accountCmd.AddCommand(inviteAccountCmd)
	inviteAccountCmd.Flags().StringVarP(&acctInviteEmail, "email", "e", "", "Invitee email address")
	inviteAccountCmd.Flags().StringSliceVarP(&acctCapabilities, "capability", "c", nil,
		"Capability to grant on accept (repeatable). Run 'ivcap capabilities --kind account' to list valid values")

	accountCmd.AddCommand(invitationsAccountCmd)
}

// acctUserFromFlag resolves the required --user URN (through GetHistory for @N).
func acctUserFromFlag() (string, error) {
	if acctPrincipalUser == "" {
		return "", fmt.Errorf("please provide the target --user")
	}
	return GetHistory(acctPrincipalUser), nil
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

	membersAccountCmd = &cobra.Command{
		Use:   "members account_id",
		Short: "List an account's members and their capabilities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			res, err := sdk.ListAccountMembersRaw(context.Background(), id, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json":
				return a.ReplyPrinter(res, false)
			case "yaml":
				return a.ReplyPrinter(res, true)
			default:
				var list accountsapi.ListMembersResult
				if err = res.AsType(&list); err != nil {
					return err
				}
				printMemberTable(list.Members)
			}
			return nil
		},
	}

	grantAccountCmd = &cobra.Command{
		Use:   "grant account_id --user <urn> --capability <cap> ...",
		Short: "Grant account capabilities to a user",
		Long: `Grant one or more account-admin capabilities to an existing member. List the
grantable capabilities with 'ivcap capabilities --kind account'.`,
		Args: cobra.ExactArgs(1),
		RunE: runAccountGrant,
	}

	revokeAccountCapabilityCmd = &cobra.Command{
		Use:     "revoke-capability account_id --user <urn> --capability <cap> ...",
		Aliases: []string{"revoke-cap"},
		Short:   "Revoke one or more capabilities from an account member",
		Long: `Revoke individual account capabilities from a user, leaving their remaining
capabilities intact. To remove a member from the account entirely, use
'ivcap account remove-member'.`,
		Args: cobra.ExactArgs(1),
		RunE: runAccountRevokeCapability,
	}

	removeAccountMemberCmd = &cobra.Command{
		Use:   "remove-member account_id --user <urn>",
		Short: "Remove a user from an account entirely (revokes all their grants)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			accountID := GetHistory(args[0])
			userID, err := acctUserFromFlag()
			if err != nil {
				return err
			}
			if _, err := sdk.RemoveAccountMemberRaw(context.Background(), accountID, userID, CreateAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Removed %s from account %s\n", userID, accountID)
			}
			return nil
		},
	}

	inviteAccountCmd = &cobra.Command{
		Use:   "invite account_id --email <email> [--capability <cap> ...]",
		Short: "Invite a user to an account",
		Long: `Invite a user (by email) to an account, granting the given capabilities when they
accept. Omit --capability for a read-only member. If no capabilities are given and
you are on an interactive terminal, you will be prompted to choose.`,
		Args: cobra.ExactArgs(1),
		RunE: runAccountInvite,
	}

	invitationsAccountCmd = &cobra.Command{
		Use:   "invitations account_id",
		Short: "List the pending invitations on an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			res, err := sdk.ListAccountInvitationsRaw(context.Background(), id, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json":
				return a.ReplyPrinter(res, false)
			case "yaml":
				return a.ReplyPrinter(res, true)
			default:
				var list accountsapi.ListInvitationsResult
				if err = res.AsType(&list); err != nil {
					return err
				}
				printInvitationTable(list.Invitations)
			}
			return nil
		},
	}
)

func runAccountGrant(cmd *cobra.Command, args []string) error {
	accountID := GetHistory(args[0])
	userID, err := acctUserFromFlag()
	if err != nil {
		return err
	}
	caps, err := resolveCapabilities("account", acctCapabilities)
	if err != nil {
		return err
	}
	if len(caps) == 0 {
		return fmt.Errorf("provide at least one --capability to grant")
	}
	req := &accountsapi.AddAccountGrantPayload2{UserId: userID, Capabilities: caps}
	if _, err := sdk.GrantAccountRaw(context.Background(), accountID, req, CreateAdapter(true), logger); err != nil {
		return err
	}
	if !silent {
		fmt.Printf("Granted [%s] to %s on account %s\n", strings.Join(caps, ", "), userID, accountID)
	}
	return nil
}

func runAccountRevokeCapability(cmd *cobra.Command, args []string) error {
	accountID := GetHistory(args[0])
	userID, err := acctUserFromFlag()
	if err != nil {
		return err
	}
	if len(acctCapabilities) == 0 {
		return fmt.Errorf("provide at least one --capability to revoke")
	}
	if _, err := resolveCapabilities("account", acctCapabilities); err != nil {
		return err
	}
	adpt := CreateAdapter(true)
	var failed []string
	for _, c := range acctCapabilities {
		if _, err := sdk.RemoveAccountGrantRaw(context.Background(), accountID, userID, c, adpt, logger); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", c, err))
			continue
		}
		if !silent {
			fmt.Printf("Revoked %s from %s on account %s\n", c, userID, accountID)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to revoke: %s", strings.Join(failed, "; "))
	}
	return nil
}

func runAccountInvite(cmd *cobra.Command, args []string) error {
	accountID := GetHistory(args[0])
	if acctInviteEmail == "" {
		return fmt.Errorf("please provide the invitee's --email")
	}
	caps, err := resolveCapabilities("account", acctCapabilities)
	if err != nil {
		return err
	}
	req := &accountsapi.CreateAccountInvitationPayload2{Email: acctInviteEmail}
	if len(caps) > 0 {
		req.Capabilities = &caps
	}
	res, err := sdk.CreateAccountInvitationRaw(context.Background(), accountID, req, CreateAdapter(true), logger)
	if err != nil {
		return err
	}
	return a.ReplyPrinter(res, outputFormat == "yaml")
}

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
