package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
)

const (
	skillsManifestURI = "skills://manifest"
	skillsCatalogURI  = "skills://catalog.json"
	skillsContextURI  = "skills://CONTEXT.md"
)

type skillsManifestItem struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// addSkillsResourcesAndPrompts exposes embedded `skills/*.SKILL.md` as MCP resources
// (MCP-Provisioned Skills) and provides a setup prompt that tells agents how
// to discover and use them.
func addSkillsResourcesAndPrompts(s *server.MCPServer) {
	if s == nil {
		return
	}

	// ---- Resources ------------------------------------------------------------------------

	// Manifest: cheap discovery of available skills.
	s.AddResource(
		mcp.NewResource(
			skillsManifestURI,
			"IVCAP skills manifest",
			mcp.WithResourceDescription("JSON manifest of skills embedded in this ivcap-cli release. Each item points to a skills://{name}/SKILL.md resource."),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
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
			b, err := json.Marshal(map[string]any{"skills": items})
			if err != nil {
				return nil, err
			}
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: skillsManifestURI, MIMEType: "application/json", Text: string(b)}}, nil
		},
	)

	// Catalog: slightly richer, still compact, intended for quick searching.
	s.AddResource(
		mcp.NewResource(
			skillsCatalogURI,
			"IVCAP skills catalog",
			mcp.WithResourceDescription("JSON catalog of embedded skills including metadata and content hashes (no full markdown bodies)."),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
			if err != nil {
				return nil, err
			}
			// omit full markdown to keep it compact
			items := make([]*skillsdoc.SkillDoc, 0, len(docs))
			for _, d := range docs {
				items = append(items, &skillsdoc.SkillDoc{HeadMatter: d.HeadMatter, Path: d.Path, SHA256: d.SHA256})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
			b, err := json.Marshal(map[string]any{"skills": items})
			if err != nil {
				return nil, err
			}
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: skillsCatalogURI, MIMEType: "application/json", Text: string(b)}}, nil
		},
	)

	// Context: general agent guidance.
	s.AddResource(
		mcp.NewResource(
			skillsContextURI,
			"IVCAP agent context",
			mcp.WithResourceDescription("General, agent-oriented operational guidance for ivcap-cli."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			b, err := fs.ReadFile(asset.FS, "CONTEXT.md")
			if err != nil {
				return nil, err
			}
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: skillsContextURI, MIMEType: "text/markdown", Text: string(b)}}, nil
		},
	)

	// Skill bodies as a template. (Fetch full body only when needed.)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"skills://{name}/SKILL.md",
			"IVCAP skill doc",
			mcp.WithTemplateDescription("Markdown skill playbook (YAML front-matter + instructions)."),
			mcp.WithTemplateMIMEType("text/markdown"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// In resource templates, matched variables are converted to request.Params.Arguments.
			// Guard nil-map to avoid panics if the server changes.
			if req.Params.Arguments == nil {
				return nil, fmt.Errorf("missing arguments")
			}
			nameAny := req.Params.Arguments["name"]
			name, _ := nameAny.(string)
			if name == "" {
				return nil, fmt.Errorf("missing name")
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
			uri := skillDocURI(name)
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: d.Content}}, nil
		},
	)

	// ---- Prompts --------------------------------------------------------------------------

	// Setup prompt: tells the agent how to self-load manifest + relevant skills for the session.
	s.AddPrompt(
		mcp.NewPrompt(
			"use-ivcap-best-practices",
			mcp.WithPromptDescription("Setup prompt: instructs the agent to read IVCAP agent context + discover and apply relevant embedded skill playbooks via MCP resources."),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			text := strings.TrimSpace(`You are operating with ivcap-cli.

For this session, load and follow IVCAP agent best-practices and any relevant skill playbooks provided by this MCP server:

1) Read the general agent guidance:
   - resources/read uri="skills://CONTEXT.md"

2) Discover available skills:
   - resources/read uri="skills://manifest"

3) Choose the most relevant skill(s) for the user’s goal and read their bodies:
   - resources/read uri="skills://{name}/SKILL.md"

4) Follow the instructions in those skill docs strictly (output json, headless auth, confirm mutations, etc.).
`)
			return &mcp.GetPromptResult{
				Description: "Load IVCAP agent context + relevant embedded skill playbooks",
				Messages: []mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
				},
			}, nil
		},
	)
}

func skillDocURI(name string) string {
	return fmt.Sprintf("skills://%s/SKILL.md", name)
}

func isSafeSkillName(s string) bool {
	// Allowed pattern: [a-z0-9-]+ (matches current skill file naming)
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}
	return true
}
