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

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

type whoamiIdentity struct {
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

type whoamiOutput struct {
	Identity           whoamiIdentity `json:"identity"`
	CurrentProject     string         `json:"current_project,omitempty"`
	CurrentProjectName string         `json:"current_project_name,omitempty"`
	AccountID          string         `json:"account_id,omitempty"`
	AccountName        string         `json:"account_name,omitempty"`
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated identity, current project and account",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctxt := GetActiveContext()
		if !IsAuthorised() {
			if outputFormat == "json" || outputFormat == "yaml" {
				return fmt.Errorf("not logged in")
			}
			if !silent {
				fmt.Println("Not logged in — run 'ivcap context login'")
			}
			return nil
		}

		out := whoamiOutput{
			Identity: whoamiIdentity{
				Email:    ctxt.Email,
				Name:     ctxt.AccountName,
				Nickname: ctxt.AccountNickName,
			},
			CurrentProject: ctxt.CurrentProject,
			AccountID:      ctxt.AccountID,
		}

		// Best-effort: add the names of the current project and account. If the
		// accounts service is unreachable still show the local identity.
		adpt := GetIdentityAdapter(true)
		if ctxt.CurrentProject != "" {
			if p, err := sdk.ReadProject(context.Background(), ctxt.CurrentProject, adpt, logger); err == nil {
				out.CurrentProjectName = p.Name
			} else {
				logger.Warn("whoami: could not read current project", log.Error(err))
			}
		}
		if ctxt.AccountID != "" {
			if res, err := sdk.ReadAccountRaw(context.Background(), ctxt.AccountID, adpt, logger); err == nil {
				var acc accountsapi.Account
				if err = res.AsType(&acc); err == nil {
					out.AccountName = acc.Name
				}
			} else {
				logger.Warn("whoami: could not read account", log.Error(err))
			}
		}

		switch outputFormat {
		case "json", "yaml":
			s, err := a.ToString(out, outputFormat == "yaml")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", s)
			return nil
		default:
			printWhoami(&out)
			return nil
		}
	},
}

func printWhoami(out *whoamiOutput) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	rows := []table.Row{}
	if out.Identity.Email != "" {
		rows = append(rows, table.Row{"Email", out.Identity.Email})
	}
	if out.Identity.Name != "" {
		rows = append(rows, table.Row{"Name", out.Identity.Name})
	}
	if out.Identity.Nickname != "" {
		rows = append(rows, table.Row{"Nickname", out.Identity.Nickname})
	}
	if out.CurrentProject != "" {
		rows = append(rows, table.Row{"Current Project", whoamiTarget(out.CurrentProjectName, out.CurrentProject)})
	}
	if out.AccountID != "" {
		rows = append(rows, table.Row{"Current Account", whoamiTarget(out.AccountName, out.AccountID)})
	}
	tw.AppendRows(rows)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})
	fmt.Printf("\n%s\n", tw.Render())

	fmt.Println()
}

// whoamiTarget renders "name  urn (@N)" so the id can be reused via the history
// token in later commands.
func whoamiTarget(name, id string) string {
	token := MakeHistory(&id)
	if token == id {
		token = ""
	} else {
		token = " (" + token + ")"
	}
	if name == "" {
		return id + token
	}
	return fmt.Sprintf("%s  %s%s", name, id, token)
}
