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

var capabilitiesKind string

func init() {
	rootCmd.AddCommand(capabilitiesCmd)
	capabilitiesCmd.Flags().StringVar(&capabilitiesKind, "kind", "", "Limit to a single kind: project | account")
}

var capabilitiesCmd = &cobra.Command{
	Use:     "capabilities",
	Aliases: []string{"cap", "caps"},
	Short:   "List the capabilities grantable on projects and accounts",
	Long: `List the grantable capabilities per target kind (project, account), as defined by
the platform authorization model. These are the values accepted by the --capability
flag of 'ivcap project grant', 'ivcap account grant', and the 'invite' commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		caps, err := sdk.GetCapabilities(context.Background(), GetIdentityAdapter(false), logger)
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

// resolveCapabilities validates the supplied capabilities against the valid set for
// the target kind, prompting interactively when none were supplied and we are on a
// TTY. It is shared by the project/account grant and invite commands.
func resolveCapabilities(kind string, supplied []string) ([]string, error) {
	caps, err := sdk.GetCapabilities(context.Background(), GetIdentityAdapter(false), logger)
	if err != nil {
		return nil, fmt.Errorf("could not load capabilities: %w", err)
	}
	valid := sdk.CapabilitiesForKind(caps, kind)

	selected := supplied
	if len(selected) == 0 && isInteractive() {
		if selected, err = selectCapabilitiesInteractive(kind, valid); err != nil {
			return nil, err
		}
	}
	if err := validateCapabilities(selected, valid); err != nil {
		return nil, err
	}
	return selected, nil
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
