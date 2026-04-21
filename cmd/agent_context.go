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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	asset "github.com/ivcap-works/ivcap-cli/skills"
	"github.com/spf13/cobra"
)

type AgentContextDoc struct {
	// Path is a stable, repo-like path for tooling.
	Path string `json:"path" yaml:"path"`

	// SHA256 is the hex digest of the full file content.
	SHA256 string `json:"sha256" yaml:"sha256"`

	// Content is the embedded markdown.
	Content string `json:"content" yaml:"content"`
}

const embeddedAgentContextFile = "CONTEXT.md"

func loadAgentContextDoc() (*AgentContextDoc, error) {
	b, err := fs.ReadFile(asset.FS, embeddedAgentContextFile)
	if err != nil {
		return nil, err
	}
	sha := sha256.Sum256(b)
	return &AgentContextDoc{
		Path:    "skills/" + embeddedAgentContextFile,
		SHA256:  hex.EncodeToString(sha[:]),
		Content: string(b),
	}, nil
}

func runAgentContext(cmd *cobra.Command) error {
	doc, err := loadAgentContextDoc()
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json", "yaml":
		payload, err := adpt.JsonPayloadFromAny(doc, logger)
		if err != nil {
			return err
		}
		return adpt.ReplyPrinter(payload, outputFormat == "yaml")
	default:
		if _, err := fmt.Fprint(os.Stdout, doc.Content); err != nil {
			return err
		}
		if len(doc.Content) == 0 || doc.Content[len(doc.Content)-1] != '\n' {
			if _, err := fmt.Fprint(os.Stdout, "\n"); err != nil {
				return err
			}
		}
		return nil
	}
}

func init() {
	rootCmd.AddCommand(agentContextCmd)
}

var agentContextCmd = &cobra.Command{
	Use:     "agent-context",
	Aliases: []string{"agent-help"},
	Short:   "Print embedded agent context guidance (markdown)",
	GroupID: agentSupportGroupID,
	Long: `Prints agent-oriented operational guidance embedded into this CLI release.

This is intended to be stable and offline-friendly for LLM/agent tooling.

Examples:
  ivcap --agent-context
  ivcap --output json --agent-context
  ivcap agent-context
`,
	Args: cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runAgentContext(cmd)
	},
}
