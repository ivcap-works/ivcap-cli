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
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(invitationCmd)

	invitationCmd.AddCommand(listInvitationCmd)
	invitationCmd.AddCommand(acceptInvitationCmd)
	invitationCmd.AddCommand(declineInvitationCmd)
	invitationCmd.AddCommand(revokeInvitationCmd)
}

var (
	invitationCmd = &cobra.Command{
		Use:     "invitation",
		Aliases: []string{"inv", "invitations"},
		Short:   "Respond to invitations addressed to you",
		Long: `Manage invitations addressed to you: 'list' the ones awaiting your response, then
'accept' or 'decline' them. 'revoke' cancels an invitation you issued.

To invite someone into a project or account, use 'ivcap project invite' or
'ivcap account invite'.`,
	}

	listInvitationCmd = &cobra.Command{
		Use:   "list",
		Short: "List invitations addressed to you",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := sdk.ListMyInvitationsRaw(context.Background(), CreateAdapter(true), logger)
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

	acceptInvitationCmd = &cobra.Command{
		Use:   "accept invitation_id",
		Short: "Accept an invitation addressed to you",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			if _, err := sdk.AcceptInvitationRaw(context.Background(), id, CreateAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Accepted invitation %s\n", id)
			}
			return nil
		},
	}

	declineInvitationCmd = &cobra.Command{
		Use:   "decline invitation_id",
		Short: "Decline an invitation addressed to you",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			if _, err := sdk.DeclineInvitationRaw(context.Background(), id, CreateAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Declined invitation %s\n", id)
			}
			return nil
		},
	}

	revokeInvitationCmd = &cobra.Command{
		Use:   "revoke invitation_id",
		Short: "Cancel a pending invitation you issued",
		Long: `Cancel a pending invitation you issued to a project or account. List the
outstanding invitations on a target with 'ivcap project invitations <project>' or
'ivcap account invitations <account>' to find the id.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			if _, err := sdk.RevokeInvitationRaw(context.Background(), id, CreateAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Revoked invitation %s\n", id)
			}
			return nil
		},
	}
)

func printInvitationTable(invitations []accountsapi.Invitation) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Kind", "Target", "Status", "Capabilities", "Expires"})
	rows := make([]table.Row, len(invitations))
	for i, inv := range invitations {
		id := inv.Id
		target := ""
		if inv.ProjectId != nil {
			target = *inv.ProjectId
		} else if inv.AccountId != nil {
			target = *inv.AccountId
		}
		rows[i] = table.Row{
			MakeHistory(&id),
			inv.Kind,
			target,
			inv.Status,
			strings.Join(inv.Capabilities, ", "),
			inv.ExpiresAt,
		}
	}
	t.AppendRows(rows)
	t.Style().Options.SeparateRows = true
	t.Render()
}
