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
	projectName      string
	projectAccountID string
)

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.AddCommand(listProjectCmd)
	addListFlags(listProjectCmd)

	projectCmd.AddCommand(readProjectCmd)

	projectCmd.AddCommand(createProjectCmd)
	createProjectCmd.Flags().StringVarP(&projectName, "name", "n", "", "Display name for the new project")
	createProjectCmd.Flags().StringVar(&projectAccountID, "account-id", "", "Owning account URN")

	projectCmd.AddCommand(useProjectCmd)
	projectCmd.AddCommand(leaveProjectCmd)
	projectCmd.AddCommand(deleteProjectCmd)

	projectCmd.AddCommand(membersProjectCmd)

	projectCmd.AddCommand(grantProjectCmd)
	addPrincipalFlags(grantProjectCmd)
	grantProjectCmd.Flags().StringSliceVarP(&projCapabilities, "capability", "c", nil,
		"Capability to grant (repeatable). Run 'ivcap capabilities --kind project' to list valid values")

	projectCmd.AddCommand(revokeProjectCapabilityCmd)
	addPrincipalFlags(revokeProjectCapabilityCmd)
	revokeProjectCapabilityCmd.Flags().StringSliceVarP(&projCapabilities, "capability", "c", nil,
		"Capability to revoke (repeatable)")

	projectCmd.AddCommand(removeProjectMemberCmd)
	addPrincipalFlags(removeProjectMemberCmd)

	projectCmd.AddCommand(inviteProjectCmd)
	inviteProjectCmd.Flags().StringVarP(&projInviteEmail, "email", "e", "", "Invitee email address")
	inviteProjectCmd.Flags().StringSliceVarP(&projCapabilities, "capability", "c", nil,
		"Capability to grant on accept (repeatable). Run 'ivcap capabilities --kind project' to list valid values")

	projectCmd.AddCommand(invitationsProjectCmd)
}

// principal selection flags shared across the project grant/revoke/remove-member
// subcommands (only one subcommand runs per invocation, so sharing the backing
// vars is safe).
var (
	projPrincipalUser    string
	projPrincipalService string
	projCapabilities     []string
	projInviteEmail      string
)

// addPrincipalFlags registers the mutually-exclusive --user/--service flags.
func addPrincipalFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&projPrincipalUser, "user", "", "User URN to target")
	cmd.Flags().StringVar(&projPrincipalService, "service", "", "Service principal URN to target")
}

// principalFromFlags resolves the (kind, id) principal from the mutually-exclusive
// --user/--service flags, requiring exactly one. The id is resolved through
// GetHistory so @N shortcuts work.
func principalFromFlags() (kind, id string, err error) {
	switch {
	case projPrincipalUser != "" && projPrincipalService != "":
		return "", "", fmt.Errorf("provide only one of --user or --service")
	case projPrincipalUser != "":
		return "user", GetHistory(projPrincipalUser), nil
	case projPrincipalService != "":
		return "service", GetHistory(projPrincipalService), nil
	default:
		return "", "", fmt.Errorf("provide exactly one of --user or --service")
	}
}

var (
	projectCmd = &cobra.Command{
		Use:     "project",
		Aliases: []string{"p", "projects"},
		Short:   "Manage projects and select the current one",
	}

	listProjectCmd = &cobra.Command{
		Use:   "list",
		Short: "List projects you can access",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := createListRequest()
			res, err := sdk.ListProjectsRaw(context.Background(), req, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json":
				return a.ReplyPrinter(res, false)
			case "yaml":
				return a.ReplyPrinter(res, true)
			default:
				var list accountsapi.ListProjectsResult
				if err = res.AsType(&list); err != nil {
					return err
				}
				printProjectTable(list.Projects)
			}
			return nil
		},
	}

	readProjectCmd = &cobra.Command{
		Use:     "get [flags] project_id",
		Aliases: []string{"read"},
		Short:   "Fetch details about a single project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			res, err := sdk.ReadProjectRaw(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			switch outputFormat {
			case "json", "yaml":
				return a.ReplyPrinter(res, outputFormat == "yaml")
			default:
				var p accountsapi.Project
				if err = res.AsType(&p); err != nil {
					return err
				}
				printProject(&p)
			}
			return nil
		},
	}

	createProjectCmd = &cobra.Command{
		Use:   "create --name <name> [--account-id <urn>]",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectName == "" {
				return fmt.Errorf("please provide a name via --name")
			}
			req := &accountsapi.CreateProjectPayload2{Name: projectName}
			if projectAccountID != "" {
				req.AccountId = &projectAccountID
			}
			res, err := sdk.CreateProjectRaw(context.Background(), req, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}

	useProjectCmd = &cobra.Command{
		Use:   "use [project_id]",
		Short: "Set the current project for this context (interactive picker if no id given)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxt := GetActiveContext()
			if len(args) == 0 {
				return selectProjectInteractive(ctxt)
			}
			id := GetHistory(args[0])
			p, err := sdk.ReadProject(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return fmt.Errorf("cannot select project %s: %w", id, err)
			}
			return setCurrentProject(ctxt, p)
		},
	}

	leaveProjectCmd = &cobra.Command{
		Use:   "leave project_id",
		Short: "Leave a project (relinquish your grants)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			_, err := sdk.LeaveProjectRaw(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Left project %s\n", id)
			}
			return nil
		},
	}

	deleteProjectCmd = &cobra.Command{
		Use:     "delete project_id",
		Aliases: []string{"remove"},
		Short:   "Delete a project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			_, err := sdk.DeleteProjectRaw(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Deleted project %s\n", id)
			}
			return nil
		},
	}

	membersProjectCmd = &cobra.Command{
		Use:   "members project_id",
		Short: "List a project's members and their capabilities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			res, err := sdk.ListProjectMembersRaw(context.Background(), id, GetIdentityAdapter(true), logger)
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

	grantProjectCmd = &cobra.Command{
		Use:   "grant project_id (--user <urn> | --service <urn>) --capability <cap> ...",
		Short: "Grant project capabilities to a user or service principal",
		Long: `Grant one or more project capabilities to an existing member (user or service
principal). List the grantable capabilities with 'ivcap capabilities --kind project'.`,
		Args: cobra.ExactArgs(1),
		RunE: runProjectGrant,
	}

	revokeProjectCapabilityCmd = &cobra.Command{
		Use:     "revoke-capability project_id (--user <urn> | --service <urn>) --capability <cap> ...",
		Aliases: []string{"revoke-cap"},
		Short:   "Revoke one or more capabilities from a project principal",
		Long: `Revoke individual project capabilities from a principal, leaving their remaining
capabilities intact. To remove a principal from the project entirely, use
'ivcap project remove-member'.`,
		Args: cobra.ExactArgs(1),
		RunE: runProjectRevokeCapability,
	}

	removeProjectMemberCmd = &cobra.Command{
		Use:   "remove-member project_id (--user <urn> | --service <urn>)",
		Short: "Remove a principal from a project entirely (revokes all their capabilities)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := GetHistory(args[0])
			kind, principalID, err := principalFromFlags()
			if err != nil {
				return err
			}
			if _, err := sdk.RemoveProjectMemberRaw(context.Background(), projectID, principalID, kind, GetIdentityAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Removed %s from project %s\n", principalID, projectID)
			}
			return nil
		},
	}

	inviteProjectCmd = &cobra.Command{
		Use:   "invite project_id --email <email> [--capability <cap> ...]",
		Short: "Invite a user to a project",
		Long: `Invite a user (by email) to a project, granting the given capabilities when they
accept. If no capabilities are given and you are on an interactive terminal, you
will be prompted to choose from the valid set.`,
		Args: cobra.ExactArgs(1),
		RunE: runProjectInvite,
	}

	invitationsProjectCmd = &cobra.Command{
		Use:   "invitations project_id",
		Short: "List the pending invitations on a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := GetHistory(args[0])
			res, err := sdk.ListProjectInvitationsRaw(context.Background(), id, GetIdentityAdapter(true), logger)
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

func runProjectGrant(cmd *cobra.Command, args []string) error {
	projectID := GetHistory(args[0])
	kind, principalID, err := principalFromFlags()
	if err != nil {
		return err
	}
	caps, err := resolveCapabilities("project", projCapabilities)
	if err != nil {
		return err
	}
	if len(caps) == 0 {
		return fmt.Errorf("provide at least one --capability to grant")
	}
	req := &accountsapi.AddProjectGrantPayload2{
		PrincipalKind: kind,
		PrincipalId:   principalID,
		Capabilities:  caps,
	}
	if _, err := sdk.GrantProjectRaw(context.Background(), projectID, req, GetIdentityAdapter(true), logger); err != nil {
		return err
	}
	if !silent {
		fmt.Printf("Granted [%s] to %s on project %s\n", strings.Join(caps, ", "), principalID, projectID)
	}
	return nil
}

func runProjectRevokeCapability(cmd *cobra.Command, args []string) error {
	projectID := GetHistory(args[0])
	kind, principalID, err := principalFromFlags()
	if err != nil {
		return err
	}
	if len(projCapabilities) == 0 {
		return fmt.Errorf("provide at least one --capability to revoke")
	}
	// Validate the requested capabilities against the project vocabulary before
	// issuing any request (never prompts: capabilities were supplied).
	if _, err := resolveCapabilities("project", projCapabilities); err != nil {
		return err
	}
	adpt := GetIdentityAdapter(true)
	var failed []string
	for _, c := range projCapabilities {
		if _, err := sdk.RemoveProjectGrantRaw(context.Background(), projectID, principalID, kind, c, adpt, logger); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", c, err))
			continue
		}
		if !silent {
			fmt.Printf("Revoked %s from %s on project %s\n", c, principalID, projectID)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to revoke: %s", strings.Join(failed, "; "))
	}
	return nil
}

func runProjectInvite(cmd *cobra.Command, args []string) error {
	projectID := GetHistory(args[0])
	if projInviteEmail == "" {
		return fmt.Errorf("please provide the invitee's --email")
	}
	caps, err := resolveCapabilities("project", projCapabilities)
	if err != nil {
		return err
	}
	req := &accountsapi.CreateInvitationPayload2{Email: projInviteEmail, Capabilities: caps}
	res, err := sdk.CreateProjectInvitationRaw(context.Background(), projectID, req, GetIdentityAdapter(true), logger)
	if err != nil {
		return err
	}
	return a.ReplyPrinter(res, outputFormat == "yaml")
}

// setCurrentProject persists the selected project (and its account) to the active
// context so subsequent authenticated requests carry the Ivcap-Project header.
func setCurrentProject(ctxt *Context, p *accountsapi.Project) error {
	ctxt.CurrentProject = p.Id
	ctxt.AccountID = p.AccountId
	SetContext(ctxt, true)
	if !silent {
		fmt.Printf("Using project %s (%s)\n", p.Name, p.Id)
	}
	return nil
}

// selectProjectInteractive lists the user's projects and prompts for one, then
// records it as the current project. It is used both by the post-login onboarding
// step and by `project use` with no argument. It returns an error (rather than
// prompting) when not attached to an interactive terminal; the login flow treats
// that error as "skip onboarding".
func selectProjectInteractive(ctxt *Context) error {
	if accessTokenProvided {
		return fmt.Errorf("a token was supplied via flag/env; select a project explicitly with 'ivcap project use <id>'")
	}
	if silent || !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("not an interactive terminal; select a project with 'ivcap project use <id>'")
	}

	res, err := sdk.ListProjects(context.Background(), &sdk.ListRequest{Limit: 100}, GetIdentityAdapter(true), logger)
	if err != nil {
		return err
	}
	projects := res.Projects
	switch len(projects) {
	case 0:
		fmt.Println("No projects available yet. A personal project is normally provisioned on first login;")
		fmt.Println("try re-running 'ivcap context login', or create one with 'ivcap project create --name <name>'.")
		return nil
	case 1:
		return setCurrentProject(ctxt, &projects[0])
	}

	fmt.Println("Select a project to use:")
	for i, p := range projects {
		fmt.Printf("  [%d] %s  (%s)\n", i+1, p.Name, p.Id)
	}
	fmt.Print("Enter number: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(projects) {
		return fmt.Errorf("invalid selection %q", line)
	}
	return setCurrentProject(ctxt, &projects[n-1])
}

// printMemberTable renders a project's or account's members. Shared by the
// 'project members' and 'account members' commands.
func printMemberTable(members []accountsapi.MemberProfile) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Kind", "Name", "Email", "Capabilities"})
	rows := make([]table.Row, len(members))
	for i, m := range members {
		id := m.UserId
		rows[i] = table.Row{
			MakeHistory(&id),
			m.Kind,
			truncString(m.DisplayName),
			m.Email,
			strings.Join(m.Capabilities, ", "),
		}
	}
	t.AppendRows(rows)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 5, WidthMax: 40, WidthMaxEnforcer: text.WrapSoft},
	})
	t.Style().Options.SeparateRows = true
	t.Render()
}

func printProjectTable(projects []accountsapi.Project) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Kind", "Account"})
	rows := make([]table.Row, len(projects))
	for i, p := range projects {
		id := p.Id
		rows[i] = table.Row{MakeHistory(&id), truncString(p.Name), p.Kind, p.AccountId}
	}
	t.AppendRows(rows)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, WidthMaxEnforcer: text.WrapSoft},
	})
	t.Style().Options.SeparateRows = true
	t.Render()
}

func printProject(p *accountsapi.Project) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	id := p.Id
	rows := []table.Row{
		{"Name", p.Name},
		{"ID", fmt.Sprintf("%s (%s)", p.Id, MakeHistory(&id))},
		{"Kind", p.Kind},
		{"Account", p.AccountId},
	}
	if p.OwnerUserId != "" {
		rows = append(rows, table.Row{"Owner", p.OwnerUserId})
	}
	tw.AppendRows(rows)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}
