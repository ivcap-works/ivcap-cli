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
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "go.uber.org/zap"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
)

type unifiedSearchArgs struct {
	Tools  *string `json:"tools,omitempty"`
	Skills *string `json:"skills,omitempty"`
	Limit  int     `json:"limit,omitempty"`
	Page   *string `json:"page,omitempty"`
}

// addUnifiedSearchTool adds a single "search" tool that can search both tools/services
// and embedded skills at the same time, helping prevent LLM clients from getting stuck
// in a tools-skills list loop.
func addUnifiedSearchTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tools": map[string]any{
				"type":        "string",
				"description": "Optional search string for available IVCAP services/tools. Searches service names and descriptions.",
			},
			"skills": map[string]any{
				"type":        "string",
				"description": "Optional search string for embedded skill playbooks. Free-text query; results are ranked by relevance (BM25) over skill names, descriptions and bodies, best match first.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max number of results to return per category (default: 10).",
			},
			"page": map[string]any{
				"type":        "string",
				"description": "Optional paging token for service results (skills do not support paging).",
			},
		},
	}

	tool := mcp.NewToolWithRawSchema(
		"search",
		"[UNIFIED SEARCH] Search for both IVCAP tools/services AND embedded skills in a single call. Provide 'tools' to search services, 'skills' to search skill playbooks, or both to get combined results. Returns matching tools and skills together, preventing the tools↔skills loop. SPECIAL: Use skills='*' to list ALL available skills (no auth needed).",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pyld, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed unifiedSearchArgs
		if err := pyld.AsType(&parsed); err != nil {
			return nil, err
		}

		result := map[string]any{}

		// Search tools/services if query provided
		if parsed.Tools != nil && *parsed.Tools != "" {
			tools, err := searchServices(ctx, *parsed.Tools, parsed.Limit, parsed.Page)
			if err != nil {
				// Don't fail the entire search if tools fail; just omit them
				if srvCfg.Logger != nil {
					srvCfg.Logger.Warn("tools search failed", log.Error(err), log.String("query", *parsed.Tools))
				}
			} else if tools != nil {
				result["tools"] = tools
			}
		}

		// Search skills if query provided
		if parsed.Skills != nil && *parsed.Skills != "" {
			skills, err := searchSkillsLocal(*parsed.Skills, parsed.Limit)
			if err != nil {
				if srvCfg.Logger != nil {
					srvCfg.Logger.Warn("skills search failed", log.Error(err), log.String("query", *parsed.Skills))
				}
			} else if skills != nil {
				result["skills"] = skills
			}
		}

		if len(result) == 0 {
			result["message"] = "No search queries provided. Use 'tools' and/or 'skills' parameters."
		}

		return mcp.NewToolResultJSON(result)
	}

	s.AddTool(tool, handler)
}

// searchServices queries the IVCAP platform for matching services.
func searchServices(ctx context.Context, query string, limit int, page *string) (any, error) {
	adpt, err := createAdapter(srvCfg.TimeoutSec)
	if err != nil {
		if isAuthFailure(err) {
			// Return login required instead of failing
			return map[string]any{"error": "login required"}, nil
		}
		return nil, err
	}
	ctxt, cancel := withTimeout(ctx)
	defer cancel()

	if limit == 0 {
		limit = 10
	}
	req0 := &sdk.ListRequest{Limit: limit, Page: page, Search: &query}
	res, err := listServicesRawFn(ctxt, req0, adpt, srvCfg.Logger)
	if err != nil {
		if isAuthFailure(err) {
			return map[string]any{"error": "login required"}, nil
		}
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	if o, err := res.AsObject(); err == nil {
		return o, nil
	}
	return string(res.AsBytes()), nil
}

// searchSkillsLocal ranks the embedded skills against query using BM25 (see
// skillsdoc.Search), best match first, and returns at most limit of them.
// Does NOT require authentication/platform access.
// Pass query="*" to list all available skills (sorted by name).
func searchSkillsLocal(query string, limit int) ([]*skillsManifestItem, error) {
	if limit == 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)

	// Special case: "*" means list all skills
	listAll := query == "*"
	if !listAll && query == "" {
		return nil, fmt.Errorf("empty search query (use '*' to list all skills)")
	}

	docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
	if err != nil {
		return nil, err
	}

	// LoadAllSkillDocs returns docs sorted by name, which is the order we want
	// for "list all"; otherwise rank by relevance.
	ranked := docs
	if !listAll {
		hits := skillsdoc.Search(docs, query)
		ranked = make([]*skillsdoc.SkillDoc, 0, len(hits))
		for _, h := range hits {
			ranked = append(ranked, h.Doc)
		}
	}
	if len(ranked) == 0 {
		return nil, fmt.Errorf("no skills found matching %q", query)
	}
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}

	results := make([]*skillsManifestItem, 0, len(ranked))
	for _, d := range ranked {
		results = append(results, &skillsManifestItem{
			Name:        d.Name,
			URI:         skillDocURI(d.Name),
			Version:     d.Version,
			Description: d.Description,
		})
	}
	return results, nil
}
