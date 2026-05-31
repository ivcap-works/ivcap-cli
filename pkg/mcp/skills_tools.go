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

package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
)

// addSkillsBridgeTools adds read_skill tool as a workaround
// for MCP clients that don't expose resources/read to LLMs.
// This tool duplicates functionality available via resources/read but is
// callable via tools/call.
// Note: list_skills functionality is now provided by the unified search tool.
func addSkillsBridgeTools(s *server.MCPServer) {
	// read_skill: reads a skill document by name
	readSkillSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill name (e.g., 'nextflow-build', 'ivcap-service-long-running')",
			},
		},
		"required": []any{"name"},
	}
	readSkillTool := mcp.NewToolWithRawSchema(
		"read_skill",
		"[BUILT-IN FALLBACK TOOL - Always available] Read an IVCAP skill playbook by exact name. ⚠️ CRITICAL: (1) ALWAYS call list_skills FIRST to verify the skill name exists - don't guess. (2) If you get an error, READ IT CAREFULLY - it tells you exactly what to do. Don't ignore errors or hallucinate solutions. Common errors: skill not found → check list_skills output, invalid name → copy exact name from list. Use this when resources/read is not available. Returns the full markdown skill document with instructions. Equivalent to: resources/read uri=\"skills://{name}/SKILL.md\".",
		MapToRaw(readSkillSchema),
	)
	s.AddTool(readSkillTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		nameAny, ok := args["name"]
		if !ok {
			return nil, fmt.Errorf("missing 'name' argument")
		}
		name, ok := nameAny.(string)
		if !ok {
			return nil, fmt.Errorf("'name' must be a string")
		}
		if !isSafeSkillName(name) {
			return nil, fmt.Errorf("invalid skill name: %q", name)
		}
		docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
		if err != nil {
			return nil, err
		}
		d := skillsdoc.FindByName(docs, name)
		if d == nil {
			return nil, fmt.Errorf("unknown skill %q", name)
		}
		return mcp.NewToolResultText(d.Content), nil
	})
}
