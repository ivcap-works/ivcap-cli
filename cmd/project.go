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
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	api "github.com/ivcap-works/ivcap-core-api/http/project"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.AddCommand(listProjectsCmd)

	projectCmd.AddCommand(projectInfoCmd)

	projectCmd.AddCommand(createProjectCmd)
	createProjectCmd.Flags().StringVarP(&projectName, "name", "n", "", "Name of project")
	createProjectCmd.Flags().StringVarP(&projectDetails, "details", "d", "", "Details of the project")
	createProjectCmd.Flags().StringVarP(&projectParentUrn, "parent_id", "p", "", "Project ID of the parent of this project")

	projectCmd.AddCommand(deleteProjectCmd)

	var membersCmd = &cobra.Command{Use: "members", Short: "Updates/Removes/Lists the members of a project"}
	membersCmd.AddCommand(listProjectMembersCmd)
	membersCmd.AddCommand(updateMembershipRoleCmd)
	membersCmd.AddCommand(removeMembershipRoleCmd)
	projectCmd.AddCommand(membersCmd)

	var defaultCmd = &cobra.Command{Use: "default", Short: "Gets/Sets the default project to use"}
	defaultCmd.AddCommand(getDefaultProjectCmd)
	defaultCmd.AddCommand(setDefaultProjectCmd)
	projectCmd.AddCommand(defaultCmd)

	var accountCmd = &cobra.Command{Use: "account", Short: "Gets/Sets the billing account associated with a project"}
	accountCmd.AddCommand(getAccountCmd)
	accountCmd.AddCommand(setAccountCmd)
	projectCmd.AddCommand(accountCmd)
}

var (
	projectURN       string
	userURN          string
	accountURN       string
	projectName      string
	projectDetails   string
	projectParentUrn string
	role             string
)

var validRoles = []string{"owner", "member"}

const (
	projectNameExample string = "Ice Shelf Dynamics"
	projectURNExample  string = "urn:ivcap:project:2feb717c-c3c3-4fb2-ad02-e122b22c7465"
	userURNExample     string = "urn:ivcap:user:02690b3e-6fd6-4cae-89a1-6eb61007bc8c"
)

var (
	projectCmd = &cobra.Command{
		Use:     "project",
		Aliases: []string{"p", "project"},
		Short:   "Create and manage projects ",
	}

	listProjectsCmd = &cobra.Command{
		Use:   "list",
		Short: "List existing projects",

		RunE: func(cmd *cobra.Command, args []string) error {
			if !silent {
				fmt.Printf("Listing Projects...\n")
			}
			req := &sdk.ListRequest{Page: nil, Limit: 50}
			if page != "" {
				req.Page = &page
			}
			if limit > 0 {
				req.Limit = limit
			}
			if res, err := sdk.ListProjectsRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var list api.ListResponseBody
					if err = res.AsType(&list); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printProjectsTable(&list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	createProjectCmd = &cobra.Command{
		Use:   "create project_name",
		Short: "Create a new project",
		Long: `Create a new project for use on the platform. The project requires a
		name.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf(" Please provide the project's name in quotations.\nExample: %s \"%s\"", cmd.CommandPath(), projectNameExample)
			}
			if len(args) > 1 {
				return fmt.Errorf(" Please provide the project's name in quotations.\nExample: %s \"%s\"", cmd.CommandPath(), projectNameExample)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		ArgAliases: []string{"project-name"},

		RunE: func(cmd *cobra.Command, args []string) error {
			projectName = args[0]
			if !silent {
				fmt.Printf("Creating Project with name %s...\n", projectName)
			}
			ctx := context.Background()

			req := &api.CreateProjectRequestBody{
				Name:       projectName,
				Properties: &api.ProjectPropertiesRequestBodyRequestBody{},
			}

			if projectDetails != "" {
				req.Properties.Details = &projectDetails
			}

			projectParentUrn = GetHistory(projectParentUrn)
			if projectParentUrn != "" {
				req.ParentProjectUrn = &projectParentUrn
			}
			if res, err := sdk.CreateProjectRaw(ctx, req, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var projectInfo api.ReadResponseBody
					if err = res.AsType(&projectInfo); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printProjectInformation(&projectInfo, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	projectInfoCmd = &cobra.Command{
		Use:   "info project_urn",
		Short: "Retrieve a project's Information",
		Long:  "Requests all information about a specific project",
		Args:  validateProjectURNArgument,

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			if !silent {
				fmt.Printf("Looking up the project with URN %s...\n", projectURN)
			}
			ctx := context.Background()

			if res, err := sdk.ProjectInfoRaw(ctx, projectURN, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var projectInfo api.ReadResponseBody
					if err = res.AsType(&projectInfo); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printProjectInformation(&projectInfo, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	deleteProjectCmd = &cobra.Command{
		Use:        "delete project_urn",
		Short:      "Deletes a project by project_urn",
		Long:       `Deletes a project by project_urn from the platform.`, // This will also delete all child projects
		Args:       validateProjectURNArgument,
		ArgAliases: []string{"project-id"},

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			if !silent {
				fmt.Printf("Deleting Project with urn %s...\n", projectURN)
			}
			ctx := context.Background()

			req := &sdk.DeleteProjectRequest{
				ProjectId: projectURN,
			}
			if res, err := sdk.DeleteProjectRaw(ctx, req, CreateAdapter(true), logger); err == nil {
				if res.StatusCode() == http.StatusNoContent {
					fmt.Printf("Success! Project Deleted")
				}
				return nil
			} else {
				return err
			}
		},
	}

	listProjectMembersCmd = &cobra.Command{
		Use:   "list project_urn",
		Short: "Lists the members of the specified project",
		Args:  validateProjectURNArgument,

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			projectHistoryToken := MakeHistory(&projectURN)

			if !silent {
				fmt.Printf("Listing members of project with URN %s (%s)...\n", projectURN, projectHistoryToken)
			}

			req := &sdk.ListProjectMembersRequest{ProjectURN: projectURN, Page: "", Limit: 50}
			if page != "" {
				req.Page = page
			}
			if limit > 0 {
				req.Limit = limit
			}

			if res, err := sdk.ListProjectMembersRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var list api.ListProjectMembersResponseBody
					if err = res.AsType(&list); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printMembersTable(&list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	updateMembershipRoleCmd = &cobra.Command{
		Use:   "update project_urn user_urn role",
		Short: "Changes the role of a user in the specified project",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
				return fmt.Errorf(" Please provide the project's URN, user's URN and the new role.\nExample: %s %s %s %s", cmd.CommandPath(), projectURNExample, userURN, "member")
			}

			args[0] = GetHistory(args[0])
			validated_project_urn, err := ValidateResourceURN(args[0], "project")

			if err != nil {
				return fmt.Errorf("\"%s\" is an invalid project URN.\nExample: %s", validated_project_urn, projectURNExample)
			} else {
				args[0] = validated_project_urn
			}

			args[0] = GetHistory(args[1])
			validated_user_urn, err := ValidateResourceURN(args[1], "user")

			if err != nil {
				return fmt.Errorf("\"%s\" is an invalid user URN.\nExample: %s", validated_user_urn, userURNExample)
			} else {
				args[1] = validated_user_urn
			}

			return cobra.ExactArgs(3)(cmd, args)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			userURN = args[1]
			role = args[2]

			projectHistoryToken := MakeHistory(&projectURN)
			userHistoryToken := MakeHistory(&userURN)

			if !silent {
				fmt.Printf("Changing the role of user %s (%s) to %s (%s) in project %s...\n", userURN, userHistoryToken, role, projectURN, projectHistoryToken)
			}

			// TODO: Grab this list dynamically from the backend
			if !slices.Contains(validRoles, role) {
				return fmt.Errorf(" Invalid Role. Please provide one of the following roles: %s", strings.Join(validRoles, ", "))
			}

			req := &api.UpdateMembershipRequestBody{Role: role}

			if res, err := sdk.UpdateMembershipRaw(context.Background(), projectURN, userURN, req, CreateAdapter(true), logger); err == nil {
				if res.StatusCode() == http.StatusNoContent {
					fmt.Printf("Success! User's role has been to: %s\n", role)
				}
				return nil
			} else {
				return err
			}
		},
	}

	removeMembershipRoleCmd = &cobra.Command{
		Use:   "remove project_urn user_urn role",
		Short: "Removes user from the specified project",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf(" Please provide the project's URN and user's URN.\nExample: %s %s %s", cmd.CommandPath(), projectURNExample, userURN)
			}

			args[0] = GetHistory(args[0])
			validated_project_urn, err := ValidateResourceURN(args[0], "project")

			if err != nil {
				return fmt.Errorf("\"%s\" is an invalid project URN.\nExample: %s", validated_project_urn, projectURNExample)
			} else {
				args[0] = validated_project_urn
			}

			args[1] = GetHistory(args[1])
			validated_user_urn, err := ValidateResourceURN(args[1], "user")

			if err != nil {
				return fmt.Errorf("\"%s\" is an invalid user URN.\nExample: %s", validated_user_urn, userURNExample)
			} else {
				args[1] = validated_user_urn
			}

			return cobra.ExactArgs(2)(cmd, args)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			userURN = args[1]

			projectHistoryToken := MakeHistory(&projectURN)
			userHistoryToken := MakeHistory(&userURN)

			if !silent {
				fmt.Printf("Removing user %s (%s) from project %s (%s)...\n", userURN, userHistoryToken, projectURN, projectHistoryToken)
			}

			if res, err := sdk.RemoveMembershipRaw(context.Background(), projectURN, userURN, CreateAdapter(true), logger); err == nil {
				if res.StatusCode() == http.StatusNoContent {
					fmt.Printf("Success! User Removed")
				}
				return nil
			} else {
				return err
			}
		},
	}

	getDefaultProjectCmd = &cobra.Command{
		Use:   "get",
		Short: "Returns the current default project to use when interacting with IVCAP",
		Args:  cobra.ExactArgs(0),

		RunE: func(cmd *cobra.Command, args []string) error {
			if !silent {
				fmt.Printf("Getting default project...\n")
			}

			if res, err := sdk.GetDefaultProjectRaw(context.Background(), CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var projectInfo api.ReadResponseBody
					if err = res.AsType(&projectInfo); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printProjectInformation(&projectInfo, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	setDefaultProjectCmd = &cobra.Command{
		Use:   "set project_urn",
		Short: "Sets the default project to use when interacting with IVCAP",
		Args:  validateProjectURNArgument,

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			projectHistoryToken := MakeHistory(&projectURN)
			if !silent {
				fmt.Printf("Setting default project with URN %s (%s)...\n", projectURN, projectHistoryToken)
			}

			req := &api.SetDefaultProjectRequestBody{ProjectUrn: projectURN}

			if res, err := sdk.SetDefaultProjectRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				if res.StatusCode() == http.StatusNoContent {
					fmt.Printf("Success! Default Project set to: %s\n", projectURN)
				}
				return nil
			} else {
				return err
			}
		},
	}

	getAccountCmd = &cobra.Command{
		Use:   "get project_urn",
		Short: "Returns the billing account associated with the specified project",
		Args:  validateProjectURNArgument,

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			projectHistoryToken := MakeHistory(&projectURN)

			if !silent {
				fmt.Printf("Getting the account of project %s (%s)...\n", projectURN, projectHistoryToken)
			}

			if res, err := sdk.GetProjectAccountRaw(context.Background(), projectURN, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var accountResponseBody api.ProjectAccountResponseBody
					if err = res.AsType(&accountResponseBody); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printAccountInformation(&accountResponseBody, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	setAccountCmd = &cobra.Command{
		Use:   "set project_urn account_urn",
		Short: "Sets the billing account associated with this project",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf(" Please provide the project's URN and user's URN.\nExample: %s %s %s", cmd.CommandPath(), projectURNExample, userURN)
			}

			args[0] = GetHistory(args[0])
			validated_project_urn, err := ValidateResourceURN(args[0], "project")

			if err != nil {
				return fmt.Errorf("\"%s\" is an invalid project URN.\nExample: %s", validated_project_urn, projectURNExample)
			} else {
				args[0] = validated_project_urn
			}

			args[1] = GetHistory(args[1])
			validated_user_urn, err := ValidateResourceURN(args[1], "account")

			if err != nil {
				return fmt.Errorf("\"%s\" is an invalid user URN.\nExample: %s", validated_user_urn, userURNExample)
			} else {
				args[1] = validated_user_urn
			}

			return cobra.ExactArgs(2)(cmd, args)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			projectURN = args[0]
			accountURN = args[1]

			projectHistoryToken := MakeHistory(&projectURN)
			accountHistoryToken := MakeHistory(&accountURN)

			if !silent {
				fmt.Printf("Setting account %s (%s) on project %s (%s)...\n", accountURN, accountHistoryToken, projectURN, projectHistoryToken)
			}

			req := &api.SetProjectAccountRequestBody{AccountUrn: accountURN}

			if res, err := sdk.SetProjectAccountRaw(context.Background(), projectURN, req, CreateAdapter(true), logger); err == nil {
				if res.StatusCode() == http.StatusNoContent {
					fmt.Printf("Success! Project (%s)'s account set to: %s\n", projectURN, accountURN)
				}
				return nil
			} else {
				return err
			}
		},
	}
)

func printProjectsTable(list *api.ListResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Name", "Role", "ID"})
	rows := make([]table.Row, len(list.Projects))
	for i, o := range list.Projects {
		rows[i] = table.Row{safeString(o.Name), safeString(o.Role), fmt.Sprintf("%s (%s)", safeString(o.Urn), MakeHistory(o.Urn))}
	}
	t.AppendRows(rows)
	t.Render()
}

func printProjectInformation(projectInfo *api.ReadResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Key", "Value"})

	propsTW := table.NewWriter()
	propsTW.SetStyle(table.StyleLight)
	if projectInfo.Properties != nil {
		if projectInfo.Properties.Details != nil {
			propsTW.AppendRow(table.Row{"Details", safeString(projectInfo.Properties.Details)})
		}
	}

	parentString := "No Parent"
	if projectInfo.Parent != nil {
		parentString = fmt.Sprintf("%s (%s)", *projectInfo.Parent, MakeMaybeHistory(projectInfo.Parent))
	}

	t.AppendRows([]table.Row{
		{"URN", fmt.Sprintf("%s (%s)", *projectInfo.Urn, MakeHistory(projectInfo.Urn))},
		{"Name", safeString(projectInfo.Name)},
		{"Status", safeString(projectInfo.Status)},
		{"CreatedAt", safeDate(projectInfo.CreatedAt, false)},
		{"ModifiedAt", safeDate(projectInfo.ModifiedAt, false)},
		{"Parent Project URN", parentString},
		{"Account URN", safeString(projectInfo.Account)},
		{"Additional Properties", propsTW.Render()},
	})

	t.Render()
}

func printMembersTable(list *api.ListProjectMembersResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Members", "Email", "Role"})
	rows := make([]table.Row, len(list.Members))
	for i, o := range list.Members {
		rows[i] = table.Row{fmt.Sprintf("%s (%s)", safeString(o.Urn), MakeHistory(o.Urn)), safeString(o.Email), safeString(o.Role)}
	}
	t.AppendRows(rows)
	t.Render()
}

func printAccountInformation(accountInfo *api.ProjectAccountResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendRow(table.Row{"Account", fmt.Sprintf("%s (%s)", safeString(accountInfo.AccountUrn), MakeHistory(accountInfo.AccountUrn))})
	t.Render()
}

// After login, we'll check if the user has a default project and if not, we'll create
// a project (and associated account)
func setupFirstProject(_ *cobra.Command, _ []string) {
	if res, err := sdk.GetDefaultProjectRaw(context.Background(), CreateAdapter(true), logger); err != nil {
		if _, ok := err.(*a.ResourceNotFoundError); ok {
			// The project no longer exists or hasn't been set
			fmt.Println()
			fmt.Println("This login does not have a default project set")

			// Check if the user is a part of any projects already
			req := &sdk.ListRequest{Page: nil, Limit: 10}
			if res, err = sdk.ListProjectsRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				var list api.ListResponseBody
				if err = res.AsType(&list); err != nil {
					fmt.Printf("Could not list user's projects: %s", err)
					return
				}
				var selectedProjectUrn string
				var selectedAccountUrn string
				var selectedOption int

				// Allow the user to select from their existing projects, or create
				// a new one
				if len(list.Projects) > 0 {
					fmt.Println("Select one of the following options")
					for i, project := range list.Projects {
						fmt.Printf("(%d) %s\n", i, *project.Name)
					}
					fmt.Printf("(%d) Create New Project\n", len(list.Projects))
					_, err = fmt.Scanln(&selectedOption)
					for err != nil || selectedOption < 0 || selectedOption > len(list.Projects) {
						fmt.Printf("Unknown option: %s\n", err)
						_, err = fmt.Scanln(&selectedOption)
					}
				} else {
					// User has no projects, so let's create one
					fmt.Println("No valid projects found. Creating new project...")
					selectedOption = len(list.Projects)
				}

				if selectedOption == len(list.Projects) {
					// Create a new one
					fmt.Println("Please enter a project name (required)")
					scanner := bufio.NewScanner(os.Stdin)
					scanner.Scan()
					err = scanner.Err()
					for err != nil {
						fmt.Printf("Unknown input: %s\n", err)
						scanner.Scan()
						err = scanner.Err()
					}
					projectName = scanner.Text()
					projectName = safeString(&projectName)

					fmt.Println("Please enter a project description (optional)")
					scanner.Scan()
					err = scanner.Err()
					for err != nil {
						fmt.Printf("Unknown input: %s\n", err)
						scanner.Scan()
						err = scanner.Err()
					}
					projectDetails = scanner.Text()
					projectDetails = safeString(&projectDetails)

					req := &api.CreateProjectRequestBody{
						Name: projectName,
						Properties: &api.ProjectPropertiesRequestBodyRequestBody{
							Details: &projectDetails,
						},
					}
					if res, err = sdk.CreateProjectRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
						var createdProject api.CreateProjectResponseBody
						if err = res.AsType(&createdProject); err != nil {
							fmt.Printf("Could not parse new project response: %s\n", err)
							return
						}
						fmt.Printf("Successfully created new project with name \"%s\"\n", projectName)
						selectedProjectUrn = *createdProject.Urn
						selectedAccountUrn = *createdProject.Account
					} else {
						fmt.Printf("Error: Could not create new project: %s\n", err)
					}
				} else {
					selectedProjectUrn = *list.Projects[selectedOption].Urn
					// Lookup the account urn for this project
					if res, err = sdk.ProjectInfoRaw(context.Background(), selectedProjectUrn, CreateAdapter(true), logger); err == nil {
						var selectedProjectInfo api.ReadResponseBody
						if err = res.AsType(&selectedProjectInfo); err != nil {
							fmt.Printf("Could not parse project info response: %s\n", err)
							return
						}
						selectedAccountUrn = *selectedProjectInfo.Account
					} else {
						fmt.Printf("Error: Could not lookup account information for this project: %s\n", err)
						return
					}
				}

				fmt.Printf("Setting this project as the default...")
				req := &api.SetDefaultProjectRequestBody{ProjectUrn: selectedProjectUrn}
				if res, err = sdk.SetDefaultProjectRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					if res.StatusCode() == http.StatusNoContent {
						fmt.Printf("Success! Default Project set to: %s\n", selectedProjectUrn)

						ctxt := GetActiveContext() // will always return ctxt or have already failed
						ctxt.DefaultProjectID = selectedProjectUrn
						ctxt.AccountID = selectedAccountUrn
						SetContext(ctxt, true)
					}
					return
				} else {
					fmt.Printf("Error: Could not set default project. %s\n", err)
					return
				}
			}
		}
	} else {
		// No errors, the user has a default project already set
		// Save the information into the context
		var defaultProjectInfo api.DefaultProjectResponseBody
		if err = res.AsType(&defaultProjectInfo); err != nil {
			fmt.Printf("Could not list user's projects: %s\n", err)
			return
		}

		ctxt := GetActiveContext() // will always return ctxt or have already failed
		ctxt.DefaultProjectID = *defaultProjectInfo.Urn
		ctxt.AccountID = *defaultProjectInfo.Account
		SetContext(ctxt, true)
	}
}

func validateProjectURNArgument(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf(" Please provide the project's URN.\nExample: %s %s", cmd.CommandPath(), projectURNExample)
	}

	validated_urn, err := ValidateResourceURN(GetHistory(args[0]), "project")

	if err != nil {
		return fmt.Errorf("\"%s\" is an invalid project URN.\nExample: %s", validated_urn, projectURNExample)
	} else {
		args[0] = validated_urn
	}

	return cobra.ExactArgs(1)(cmd, args)
}
