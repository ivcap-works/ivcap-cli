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
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
)

// addSkillsBridgeTools adds list_skills and read_skill tools as a workaround
// for MCP clients that don't expose resources/read to LLMs.
// These tools duplicate functionality available via resources/read but are
// callable via tools/call.
func addSkillsBridgeTools(s *server.MCPServer) {
	// list_skills: returns the skills manifest as JSON
	listSkillsSchema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	listSkillsTool := mcp.NewToolWithRawSchema(
		"list_skills",
		"[BUILT-IN FALLBACK TOOL - Always available] List all available IVCAP skill playbooks. Use this when resources/read is not available to you. No select_tools needed. Returns JSON manifest with skill names, URIs, versions, and descriptions. Equivalent to: resources/read uri=\"skills://manifest\". After listing, call read_skill to get specific skill content.",
		MapToRaw(listSkillsSchema),
	)
	s.AddTool(listSkillsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
		if err != nil {
			return nil, err
		}
		items := make([]skillsManifestItem, 0, len(docs))
		for _, d := range docs {
			items = append(items, skillsManifestItem{
				Name:        d.Name,
				URI:         skillDocURI(d.Name),
				Version:     d.Version,
				Description: d.Description,
			})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return mcp.NewToolResultJSON(map[string]any{"skills": items})
	})

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
		"[BUILT-IN FALLBACK TOOL - Always available] Read an IVCAP skill playbook by name. Use this when resources/read is not available to you. No select_tools needed. Returns the full markdown skill document with instructions. Equivalent to: resources/read uri=\"skills://{name}/SKILL.md\". First call list_skills to see available skill names.",
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
