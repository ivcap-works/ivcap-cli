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
			res, err := sdk.ListProjectsRaw(context.Background(), req, CreateAdapter(true), logger)
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
			res, err := sdk.ReadProjectRaw(context.Background(), id, CreateAdapter(true), logger)
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
			res, err := sdk.CreateProjectRaw(context.Background(), req, CreateAdapter(true), logger)
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
			p, err := sdk.ReadProject(context.Background(), id, CreateAdapter(true), logger)
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
			_, err := sdk.LeaveProjectRaw(context.Background(), id, CreateAdapter(true), logger)
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
			_, err := sdk.DeleteProjectRaw(context.Background(), id, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Deleted project %s\n", id)
			}
			return nil
		},
	}
)

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

	res, err := sdk.ListProjects(context.Background(), &sdk.ListRequest{Limit: 100}, CreateAdapter(true), logger)
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
