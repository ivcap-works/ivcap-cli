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
	"time"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

var (
	datasetProject     string
	datasetName        string
	datasetDescription string

	datasetGranteeProject string
	datasetGranteeService string
	datasetEveryone       bool
	datasetAccess         string
)

func init() {
	contextCmd.AddCommand(datasetCmd)

	datasetCmd.AddCommand(listDatasetCmd)
	datasetCmd.AddCommand(readDatasetCmd)

	datasetCmd.AddCommand(createDatasetCmd)
	createDatasetCmd.Flags().StringVarP(&datasetName, "name", "n", "", "Display name for the new dataset")
	createDatasetCmd.Flags().StringVarP(&datasetDescription, "description", "d", "", "Description")
	createDatasetCmd.Flags().StringVar(&datasetProject, "project", "", "Owning project URN (default: the current project)")

	datasetCmd.AddCommand(updateDatasetCmd)
	updateDatasetCmd.Flags().StringVarP(&datasetName, "name", "n", "", "New display name")
	updateDatasetCmd.Flags().StringVarP(&datasetDescription, "description", "d", "", "New description")

	datasetCmd.AddCommand(deleteDatasetCmd)

	datasetCmd.AddCommand(grantsDatasetCmd)

	datasetCmd.AddCommand(grantDatasetCmd)
	addDatasetGranteeFlags(grantDatasetCmd)
	datasetCmd.AddCommand(revokeDatasetCmd)
	addDatasetGranteeFlags(revokeDatasetCmd)

	datasetCmd.AddCommand(eventsDatasetCmd)
}

// addDatasetGranteeFlags registers the grantee and access flags shared by the
// grant and revoke subcommands (only one runs per invocation).
func addDatasetGranteeFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&datasetGranteeProject, "project", "", "Project URN to grant to")
	cmd.Flags().StringVar(&datasetGranteeService, "service", "", "Service principal URN to grant to")
	cmd.Flags().BoolVar(&datasetEveryone, "everyone", false, "Grant to everyone (read-only)")
	cmd.Flags().StringVar(&datasetAccess, "access", "", "Access level: read or write")
}

var (
	datasetCmd = &cobra.Command{
		Use:     "dataset",
		Aliases: []string{"ds", "datasets"},
		Short:   "Manage datasets and who can access them",
		Long: `Manage datasets. A dataset belongs to one project, which manages it; other
projects, service principals or everyone can be granted read or write access.
Every project has a default dataset that shares the project's id.

Datasets are addressed by URN (urn:ivcap:dataset:<uuid>).`,
	}

	listDatasetCmd = &cobra.Command{
		Use:   "list [project_id]",
		Short: "List the datasets a project owns or has been granted access to",
		Long:  `Lists the datasets of the given project, or of the current project if none is given.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid := ""
			if len(args) > 0 {
				pid = GetHistory(args[0])
			}
			pid, err := datasetProjectID(pid)
			if err != nil {
				return err
			}
			res, err := sdk.ListProjectDatasetsRaw(context.Background(), pid, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			if outputFormat == "json" || outputFormat == "yaml" {
				return a.ReplyPrinter(res, outputFormat == "yaml")
			}
			var list accountsapi.ListProjectDatasetsResult
			if err = res.AsType(&list); err != nil {
				return err
			}
			printDatasetTable(list.Datasets)
			return nil
		},
	}

	readDatasetCmd = &cobra.Command{
		Use:     "get dataset_urn",
		Aliases: []string{"read"},
		Short:   "Fetch details about a single dataset",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := datasetArg(args[0])
			if err != nil {
				return err
			}
			res, err := sdk.ReadDatasetRaw(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			if outputFormat == "json" || outputFormat == "yaml" {
				return a.ReplyPrinter(res, outputFormat == "yaml")
			}
			var d accountsapi.Dataset
			if err = res.AsType(&d); err != nil {
				return err
			}
			printDataset(&d)
			return nil
		},
	}

	createDatasetCmd = &cobra.Command{
		Use:   "create --name <name> [--description <text>] [--project <urn>]",
		Short: "Create a dataset owned by a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if datasetName == "" {
				return fmt.Errorf("please provide a name via --name")
			}
			pid, err := datasetProjectID(GetHistory(datasetProject))
			if err != nil {
				return err
			}
			req := &accountsapi.CreateDatasetPayload{Name: datasetName}
			if cmd.Flags().Changed("description") {
				req.Description = &datasetDescription
			}
			res, err := sdk.CreateDatasetRaw(context.Background(), pid, req, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}

	updateDatasetCmd = &cobra.Command{
		Use:   "update dataset_urn [--name <name>] [--description <text>]",
		Short: "Change a dataset's name or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := datasetArg(args[0])
			if err != nil {
				return err
			}
			req := &accountsapi.UpdateDatasetPayload{}
			if cmd.Flags().Changed("name") {
				req.Name = &datasetName
			}
			if cmd.Flags().Changed("description") {
				req.Description = &datasetDescription
			}
			if req.Name == nil && req.Description == nil {
				return fmt.Errorf("provide --name and/or --description")
			}
			res, err := sdk.UpdateDatasetRaw(context.Background(), id, req, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			return a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}

	deleteDatasetCmd = &cobra.Command{
		Use:     "delete dataset_urn",
		Aliases: []string{"remove"},
		Short:   "Delete a dataset (not yet supported)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Deliberately makes no request: it is not yet settled whether deleting
			// a dataset deletes its data or only withdraws visibility of it.
			return fmt.Errorf("deleting datasets is not supported yet")
		},
	}

	grantsDatasetCmd = &cobra.Command{
		Use:   "grants dataset_urn",
		Short: "List who can read or write a dataset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := datasetArg(args[0])
			if err != nil {
				return err
			}
			res, err := sdk.ListDatasetGrantsRaw(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			if outputFormat == "json" || outputFormat == "yaml" {
				return a.ReplyPrinter(res, outputFormat == "yaml")
			}
			var list accountsapi.ListDatasetGrantsResult
			if err = res.AsType(&list); err != nil {
				return err
			}
			printDatasetGrantTable(list.Grants)
			return nil
		},
	}

	grantDatasetCmd = &cobra.Command{
		Use:   "grant dataset_urn (--project <urn> | --service <urn> | --everyone) --access read|write",
		Short: "Grant read or write access to a dataset",
		Long: `Grant a project, a service principal or everyone access to a dataset. Granting
to everyone is read-only. Granting to a project needs share rights on the
dataset and write access on that project.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, req, err := datasetGrantRequest(args[0])
			if err != nil {
				return err
			}
			if _, err = sdk.AddDatasetGrantRaw(context.Background(), id, req, GetIdentityAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Granted %s on %s to %s\n", req.Access, id, describeDatasetGrantee(req))
			}
			return nil
		},
	}

	revokeDatasetCmd = &cobra.Command{
		Use:   "revoke dataset_urn (--project <urn> | --service <urn> | --everyone) --access read|write",
		Short: "Revoke a dataset grant",
		Long: `Revoke one access level from a grantee. Either side may revoke: share rights on
the dataset, or write access on the granted project.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, req, err := datasetGrantRequest(args[0])
			if err != nil {
				return err
			}
			if _, err = sdk.RemoveDatasetGrantRaw(context.Background(), id, req, GetIdentityAdapter(true), logger); err != nil {
				return err
			}
			if !silent {
				fmt.Printf("Revoked %s on %s from %s\n", req.Access, id, describeDatasetGrantee(req))
			}
			return nil
		},
	}

	eventsDatasetCmd = &cobra.Command{
		Use:   "events dataset_urn",
		Short: "Show a dataset's audit history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := datasetArg(args[0])
			if err != nil {
				return err
			}
			res, err := sdk.ListDatasetEventsRaw(context.Background(), id, GetIdentityAdapter(true), logger)
			if err != nil {
				return err
			}
			if outputFormat == "json" || outputFormat == "yaml" {
				return a.ReplyPrinter(res, outputFormat == "yaml")
			}
			var list accountsapi.ListDatasetEventsResult
			if err = res.AsType(&list); err != nil {
				return err
			}
			printDatasetEventTable(list.Events)
			return nil
		},
	}
)

// datasetArg resolves a dataset argument (through the @N history) and requires
// it to be a dataset URN.
func datasetArg(arg string) (string, error) {
	id := GetHistory(arg)
	if err := sdk.ValidateDatasetURN(id); err != nil {
		return "", err
	}
	return id, nil
}

// datasetProjectID returns pid, or the current project's id when pid is empty.
func datasetProjectID(pid string) (string, error) {
	if pid != "" {
		return pid, nil
	}
	if cur := GetActiveContext().CurrentProject; cur != "" {
		return cur, nil
	}
	return "", fmt.Errorf("no project given and no current project; use 'ivcap context project use' or pass a project")
}

// datasetGrantRequest builds a validated grant payload from the grantee flags,
// which must name exactly one grantee.
func datasetGrantRequest(arg string) (string, *accountsapi.DatasetGrantPayload, error) {
	id, err := datasetArg(arg)
	if err != nil {
		return "", nil, err
	}
	n := 0
	for _, set := range []bool{datasetGranteeProject != "", datasetGranteeService != "", datasetEveryone} {
		if set {
			n++
		}
	}
	if n != 1 {
		return "", nil, fmt.Errorf("provide exactly one of --project, --service or --everyone")
	}
	req := &accountsapi.DatasetGrantPayload{Access: datasetAccess}
	switch {
	case datasetGranteeProject != "":
		gid := GetHistory(datasetGranteeProject)
		req.GranteeKind, req.GranteeId = sdk.DatasetGranteeProject, &gid
	case datasetGranteeService != "":
		gid := GetHistory(datasetGranteeService)
		req.GranteeKind, req.GranteeId = sdk.DatasetGranteeService, &gid
	default:
		req.GranteeKind = sdk.DatasetGranteePublic
	}
	if err := sdk.ValidateDatasetGrant(req.GranteeKind, derefStr(req.GranteeId), req.Access); err != nil {
		return "", nil, err
	}
	return id, req, nil
}

func describeDatasetGrantee(req *accountsapi.DatasetGrantPayload) string {
	if req.GranteeId == nil {
		return "everyone"
	}
	return fmt.Sprintf("%s %s", req.GranteeKind, *req.GranteeId)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func printDataset(d *accountsapi.Dataset) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	id := d.Id
	rows := []table.Row{
		{"Name", d.Name},
		{"ID", fmt.Sprintf("%s (%s)", d.Id, MakeHistory(&id))},
		{"Project", d.ProjectId},
		{"Default", d.Default},
		{"Capability", d.Capability},
	}
	if d.Description != "" {
		rows = append(rows, table.Row{"Description", d.Description})
	}
	if d.CreatedBy != nil {
		rows = append(rows, table.Row{"Created by", *d.CreatedBy})
	}
	rows = append(rows, table.Row{"Created at", d.CreatedAt.Format(time.RFC3339)})
	tw.AppendRows(rows)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func printDatasetTable(datasets []accountsapi.ProjectDataset) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Default", "Owned", "Access", "Capability"})
	rows := make([]table.Row, len(datasets))
	for i, d := range datasets {
		id := d.Id
		rows[i] = table.Row{
			MakeHistory(&id),
			truncString(d.Name),
			d.Default,
			d.Owned,
			strings.Join(d.Access, ", "),
			d.Capability,
		}
	}
	t.AppendRows(rows)
	t.Style().Options.SeparateRows = true
	t.Render()
}

func printDatasetGrantTable(grants []accountsapi.DatasetGrant) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Grantee", "ID", "Access", "Granted by", "Granted at"})
	rows := make([]table.Row, len(grants))
	for i, g := range grants {
		rows[i] = table.Row{
			g.GranteeKind, derefStr(g.GranteeId), g.Access, derefStr(g.GrantedBy), g.GrantedAt.Format(time.RFC3339),
		}
	}
	t.AppendRows(rows)
	t.Style().Options.SeparateRows = true
	t.Render()
}

func printDatasetEventTable(events []accountsapi.DatasetEvent) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"At", "Action", "Grantee", "Access", "Actor"})
	rows := make([]table.Row, len(events))
	for i, e := range events {
		grantee := derefStr(e.GranteeKind)
		if e.GranteeId != nil {
			grantee += " " + *e.GranteeId
		}
		rows[i] = table.Row{e.At.Format(time.RFC3339), e.Action, grantee, derefStr(e.Access), derefStr(e.Actor)}
	}
	t.AppendRows(rows)
	t.Style().Options.SeparateRows = true
	t.Render()
}
