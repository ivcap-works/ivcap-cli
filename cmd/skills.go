// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
}

var skillsCmd = &cobra.Command{
	Use:     "skills",
	Short:   "List and show agent skill docs embedded in this CLI release",
	GroupID: agentSupportGroupID,
	Long: `Skills are small, version-matched markdown documents embedded into this CLI.
They are meant to be cheap and reliable for agents to access offline.

Each skill file must start with YAML front-matter. Expected head-matter schema:
  ` + skillsdoc.HeadMatterSpec + `
`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available skills",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
		if err != nil {
			return err
		}

		// For list output we do NOT include the full content (too big/noisy).
		items := make([]*skillsdoc.SkillDoc, 0, len(docs))
		for _, d := range docs {
			items = append(items, &skillsdoc.SkillDoc{
				HeadMatter: d.HeadMatter,
				Path:       d.Path,
				SHA256:     d.SHA256,
			})
		}

		switch outputFormat {
		case "json", "yaml":
			payload, err := adpt.JsonPayloadFromAny(map[string]any{"skills": items}, logger)
			if err != nil {
				return err
			}
			return adpt.ReplyPrinter(payload, outputFormat == "yaml")
		default:
			for _, s := range items {
				fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", s.Name, s.Version, s.Description)
			}
			return nil
		}
	},
}

var skillsShowCmd = &cobra.Command{
	Use:   "show <skill-name>",
	Short: "Show a skill doc (prints exact embedded SKILL.md content)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
		if err != nil {
			return err
		}
		d := skillsdoc.FindByName(docs, name)
		if d == nil {
			return fmt.Errorf("unknown skill '%s'", name)
		}

		switch outputFormat {
		case "json", "yaml":
			payload, err := adpt.JsonPayloadFromAny(d, logger)
			if err != nil {
				return err
			}
			return adpt.ReplyPrinter(payload, outputFormat == "yaml")
		default:
			fmt.Fprint(os.Stdout, d.Content)
			if len(d.Content) == 0 || d.Content[len(d.Content)-1] != '\n' {
				fmt.Fprint(os.Stdout, "\n")
			}
			return nil
		}
	},
}
