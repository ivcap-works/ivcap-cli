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
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	inviteProjectID    string
	inviteAccountID    string
	inviteEmail        string
	inviteCapabilities []string
	capabilitiesKind   string
)

func init() {
	rootCmd.AddCommand(invitationCmd)

	invitationCmd.AddCommand(listInvitationCmd)
	invitationCmd.AddCommand(acceptInvitationCmd)
	invitationCmd.AddCommand(declineInvitationCmd)

	invitationCmd.AddCommand(createInvitationCmd)
	createInvitationCmd.Flags().StringVar(&inviteProjectID, "project", "", "Project URN to invite the user into")
	createInvitationCmd.Flags().StringVar(&inviteAccountID, "account", "", "Account URN to invite the user into")
	createInvitationCmd.Flags().StringVarP(&inviteEmail, "email", "e", "", "Invitee email address")
	createInvitationCmd.Flags().StringSliceVarP(&inviteCapabilities, "capability", "c", nil,
		"Capability to grant on accept (repeatable). Run 'ivcap invitation capabilities' to list valid values")

	invitationCmd.AddCommand(capabilitiesCmd)
	capabilitiesCmd.Flags().StringVar(&capabilitiesKind, "kind", "", "Limit to a single kind: project | account")
}

var (
	invitationCmd = &cobra.Command{
		Use:     "invitation",
		Aliases: []string{"inv", "invitations"},
		Short:   "Manage project and account invitations",
		Long: `Manage invitations to projects and accounts.

An invitation grants an email address a set of capabilities on a target when they
accept it. The target is either a PROJECT (` + "`--project`" + `) or an ACCOUNT
(` + "`--account`" + `); the valid capabilities differ by target kind. List them with:

    ivcap invitation capabilities

Invitees manage invitations addressed to them with 'list', 'accept' and 'decline'.`,
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

	createInvitationCmd = &cobra.Command{
		Use:   "create (--project <urn> | --account <urn>) --email <email> [--capability <cap> ...]",
		Short: "Invite a user to a project or an account",
		Long: `Invite a user (by email) to a project or an account.

Exactly one target must be given:
  --project <urn>   invite into a project (capabilities: 'ivcap invitation capabilities --kind project')
  --account <urn>   invite into an account (capabilities: 'ivcap invitation capabilities --kind account')

Capabilities may be passed with repeated --capability flags. If none are given and
you are on an interactive terminal, you will be prompted to choose from the valid
set for the target kind. Provided capabilities are validated before the invitation
is created.`,
		RunE: runCreateInvitation,
	}

	capabilitiesCmd = &cobra.Command{
		Use:     "capabilities",
		Aliases: []string{"caps"},
		Short:   "List the capabilities that can be granted via an invitation or grant",
		Long:    "List the grantable capabilities per target kind (project, account), as defined by the platform authorization model.",
		RunE: func(cmd *cobra.Command, args []string) error {
			caps, err := sdk.GetCapabilities(context.Background(), CreateAdapter(false), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json", "yaml":
				s, err := a.ToString(caps, outputFormat == "yaml")
				if err != nil {
					return err
				}
				fmt.Printf("%s\n", s)
				return nil
			default:
				switch capabilitiesKind {
				case "project":
					printCapabilityTable("project", caps.Project)
				case "account":
					printCapabilityTable("account", caps.Account)
				case "":
					printCapabilityTable("project", caps.Project)
					printCapabilityTable("account", caps.Account)
				default:
					return fmt.Errorf("unknown --kind %q (use 'project' or 'account')", capabilitiesKind)
				}
			}
			return nil
		},
	}
)

func runCreateInvitation(cmd *cobra.Command, args []string) error {
	if (inviteProjectID == "") == (inviteAccountID == "") {
		return fmt.Errorf("provide exactly one of --project or --account")
	}
	if inviteEmail == "" {
		return fmt.Errorf("please provide the invitee's --email")
	}
	kind, targetID := "project", inviteProjectID
	if inviteAccountID != "" {
		kind, targetID = "account", inviteAccountID
	}

	// Fetch the valid capabilities for the target kind (public reference data).
	caps, err := sdk.GetCapabilities(context.Background(), CreateAdapter(false), logger)
	if err != nil {
		return fmt.Errorf("could not load capabilities: %w", err)
	}
	valid := sdk.CapabilitiesForKind(caps, kind)

	selected := inviteCapabilities
	if len(selected) == 0 && isInteractive() {
		if selected, err = selectCapabilitiesInteractive(kind, valid); err != nil {
			return err
		}
	}
	if err := validateCapabilities(selected, valid); err != nil {
		return err
	}

	ctx := context.Background()
	if kind == "project" {
		req := &accountsapi.CreateInvitationPayload2{Email: inviteEmail, Capabilities: selected}
		res, err := sdk.CreateProjectInvitationRaw(ctx, GetHistory(targetID), req, CreateAdapter(true), logger)
		if err != nil {
			return err
		}
		return a.ReplyPrinter(res, outputFormat == "yaml")
	}
	req := &accountsapi.CreateAccountInvitationPayload2{Email: inviteEmail}
	if len(selected) > 0 {
		req.Capabilities = &selected
	}
	res, err := sdk.CreateAccountInvitationRaw(ctx, GetHistory(targetID), req, CreateAdapter(true), logger)
	if err != nil {
		return err
	}
	return a.ReplyPrinter(res, outputFormat == "yaml")
}

// validateCapabilities checks each requested capability against the valid set for
// the kind, returning a helpful error that lists the accepted values.
func validateCapabilities(requested []string, valid []accountsapi.Capability) error {
	allowed := make(map[string]bool, len(valid))
	names := make([]string, len(valid))
	for i, c := range valid {
		allowed[c.Name] = true
		names[i] = c.Name
	}
	var bad []string
	for _, r := range requested {
		if !allowed[r] {
			bad = append(bad, r)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("unknown capabilit%s %s; valid values are: %s",
			plural(len(bad), "y", "ies"), strings.Join(bad, ", "), strings.Join(names, ", "))
	}
	return nil
}

// selectCapabilitiesInteractive prompts the user to pick capabilities from the
// valid set for a kind. Returns the chosen capability names (possibly empty).
func selectCapabilitiesInteractive(kind string, valid []accountsapi.Capability) ([]string, error) {
	if len(valid) == 0 {
		return nil, nil
	}
	fmt.Printf("Select %s capabilities to grant:\n", kind)
	for i, c := range valid {
		fmt.Printf("  [%d] %-14s %s\n", i+1, c.Name, safeString(c.Description))
	}
	fmt.Print("Enter numbers separated by commas (e.g. 1,2), or leave blank for none: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	var chosen []string
	seen := map[int]bool{}
	for _, tok := range strings.Split(line, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > len(valid) {
			return nil, fmt.Errorf("invalid selection %q", tok)
		}
		if !seen[n] {
			seen[n] = true
			chosen = append(chosen, valid[n-1].Name)
		}
	}
	return chosen, nil
}

// isInteractive reports whether we can prompt the user (a TTY, not silent, and no
// headless token provided).
func isInteractive() bool {
	return !silent && !accessTokenProvided && term.IsTerminal(int(os.Stdin.Fd()))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func printCapabilityTable(kind string, caps []accountsapi.Capability) {
	title := kind
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	fmt.Printf("\n%s capabilities:\n", title)
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Capability", "Description"})
	for _, c := range caps {
		t.AppendRow(table.Row{c.Name, safeString(c.Description)})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, WidthMax: 70, WidthMaxEnforcer: text.WrapSoft},
	})
	t.Render()
}

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
