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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

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
				"description": "Optional search string for embedded skill playbooks: a skill name, a keyword or a question. Searches skill names and descriptions by the words of the string, best matches first. Use '*' to list all skills.",
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
		"[UNIFIED SEARCH] Search for both IVCAP tools/services AND embedded skills in a single call. Provide 'tools' to search services, 'skills' to search skill playbooks, or both to get combined results. Returns matching tools and skills together, preventing the tools↔skills loop. SPECIAL: Use skills='*' to list ALL available skills (no auth needed). The 'tools' query also returns matching BUILT-IN tools (such as genome_list, artifact_get or aspect_search) as 'builtin_tools'. They are not platform services and are always callable directly; for the tasks they cover they are a better choice than any service that was found.",
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

		// Built-in tools are not platform services, so the service search above can never find them.
		if parsed.Tools != nil && *parsed.Tools != "" {
			if hits := builtinToolMatches(s, *parsed.Tools, parsed.Limit); len(hits) > 0 {
				result["builtin_tools"] = hits
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

// searchSkillsLocal performs substring matching against embedded skills locally.
// Does NOT require authentication/platform access.
// Pass query="*" to list all available skills.
func searchSkillsLocal(query string, limit int) ([]*skillsManifestItem, error) {
	// "*" lists every skill unless the caller gave a limit; a search returns at most 10 by default.
	explicitLimit := limit > 0
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

	// Match on the words of the query, so that a question works as well as a keyword: every skill whose
	// name or description contains one of them is returned, best matches first. The whole query found in
	// a name or description counts most, so an exact skill name comes first.
	words := searchWords(query)
	queryLower := strings.ToLower(query)
	type scoredSkill struct {
		item  *skillsManifestItem
		score int
	}
	var scored []scoredSkill
	for _, d := range docs {
		score := 1
		if !listAll {
			text := strings.ToLower(d.Name + " " + d.Description)
			score = matchScore(text, words)
			if strings.Contains(text, queryLower) {
				score += 10
			}
		}
		if score > 0 {
			scored = append(scored, scoredSkill{
				item:  &skillsManifestItem{Name: d.Name, URI: skillDocURI(d.Name), Version: d.Version, Description: d.Description},
				score: score,
			})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].item.Name < scored[j].item.Name
	})
	var results []*skillsManifestItem
	for i, r := range scored {
		if (!listAll || explicitLimit) && i >= limit {
			break
		}
		results = append(results, r.item)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no skills found matching %q", query)
	}

	return results, nil
}

// builtinToolHit is a built-in tool that matched a search.
type builtinToolHit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Note        string `json:"note"`
}

// searchStopWords are left out of a search, so that a question such as "which genomes are available"
// is matched on "genomes" and not on every tool that mentions "available".
var searchStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "what": true, "which": true, "are": true, "there": true,
	"that": true, "this": true, "with": true, "from": true, "any": true, "kind": true, "kinds": true,
	"available": true, "ivcap": true, "platform": true, "have": true, "can": true, "show": true,
}

func searchWords(query string) []string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		if len(w) < 3 || searchStopWords[w] {
			continue
		}
		words = append(words, w)
	}
	return words
}

// builtinToolMatches finds the built-in tools (the ones that are always listed, such as genome_list or
// artifact_get) that match a search question. The service search cannot return them, because they are
// not platform services. Read-only tools come first among equally good matches.
func builtinToolMatches(s *server.MCPServer, query string, limit int) []builtinToolHit {
	words := searchWords(query)
	if len(words) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}
	// Without a session the tool list is exactly the built-in tools.
	res, ok := s.HandleMessage(context.Background(),
		json.RawMessage(`{"jsonrpc":"2.0","id":0,"method":"tools/list","params":{}}`)).(mcp.JSONRPCResponse)
	if !ok {
		return nil
	}
	b, err := json.Marshal(res.Result)
	if err != nil {
		return nil
	}
	var listed struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(b, &listed); err != nil {
		return nil
	}

	type scored struct {
		hit      builtinToolHit
		score    int
		readOnly bool
	}
	var found []scored
	for _, t := range listed.Tools {
		if !builtInToolNames[t.Name] || t.Name == "search" || t.Name == "select_tools" {
			continue
		}
		text := strings.ToLower(t.Name + " " + t.Description)
		score := matchScore(text, words)
		if score > 0 {
			found = append(found, scored{
				hit:      builtinToolHit{Name: t.Name, Description: t.Description, Note: "built-in tool: call it directly, it is not a platform service"},
				score:    score,
				readOnly: strings.Contains(text, "read-only"),
			})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score > found[j].score
		}
		if found[i].readOnly != found[j].readOnly {
			return found[i].readOnly
		}
		return found[i].hit.Name < found[j].hit.Name
	})
	hits := make([]builtinToolHit, 0, len(found))
	for i, f := range found {
		if i >= limit {
			break
		}
		hits = append(hits, f.hit)
	}
	return hits
}

// matchScore rates how well a text matches the words of a question: a word found as it is counts 2, a
// plural word ("genomes") found only as its singular ("genome") counts 1.
func matchScore(text string, words []string) int {
	score := 0
	for _, w := range words {
		switch {
		case strings.Contains(text, w):
			score += 2
		case len(w) > 3 && strings.HasSuffix(w, "s") && strings.Contains(text, strings.TrimSuffix(w, "s")):
			score++
		}
	}
	return score
}
