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

package skillsdoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// HeadMatterSpec is the exact YAML front-matter schema we expect at the top
// of each *.SKILL.md file.
//
// The front-matter MUST be the first thing in the file and MUST be delimited by
// two lines containing only "---".
//
// Required keys:
//   - name: string (unique skill name; recommended to match file base name)
//   - version: string (semver-like, eg "0.1.0")
//   - description: string (one-line summary)
//   - requires:
//     bins: [string] (CLI binaries required; for now always includes "ivcap")
//
// Example:
//
// ---
// name: ivcap-job-create
// version: 0.1.0
// description: Create an IVCAP job from a service.
// requires:
//
//	bins: ["ivcap"]
//
// ---
const HeadMatterSpec = "YAML front-matter delimited by '---' with required keys: name, version, description, requires.bins[]"

type Requires struct {
	Bins []string `yaml:"bins" json:"bins"`
}

type HeadMatter struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description" json:"description"`
	Requires    Requires `yaml:"requires" json:"requires"`
}

type SkillDoc struct {
	HeadMatter

	// Path is a stable, repo-like path, eg "skills/ivcap-job-create.SKILL.md".
	Path string `json:"path" yaml:"path"`

	// SHA256 is the hex digest of the full file content (including front-matter).
	SHA256 string `json:"sha256" yaml:"sha256"`

	// Content is the full SKILL.md content (including front-matter).
	Content string `json:"content,omitempty" yaml:"content,omitempty"`
}

func ParseSkillDoc(path string, content []byte) (*SkillDoc, error) {
	sha := sha256.Sum256(content)
	doc := &SkillDoc{
		Path:    path,
		SHA256:  hex.EncodeToString(sha[:]),
		Content: string(content),
	}

	// Parse YAML front-matter.
	// We expect the file to begin with:
	// ---\n<yaml>\n---\n
	front, err := extractFrontMatter(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var hm HeadMatter
	if err := yaml.Unmarshal(front, &hm); err != nil {
		return nil, fmt.Errorf("%s: invalid YAML front-matter: %w", path, err)
	}
	if err := validateHeadMatter(path, &hm); err != nil {
		return nil, err
	}
	doc.HeadMatter = hm
	return doc, nil
}

func LoadAllSkillDocs(fsys fs.FS) ([]*SkillDoc, error) {
	entries, err := fs.Glob(fsys, "*.SKILL.md")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	out := make([]*SkillDoc, 0, len(entries))
	for _, p := range entries {
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		repoLikePath := filepath.ToSlash(filepath.Join("skills", p))
		d, err := ParseSkillDoc(repoLikePath, b)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}

	// Stable ordering for output
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func FindByName(docs []*SkillDoc, name string) *SkillDoc {
	for _, d := range docs {
		if d.Name == name {
			return d
		}
	}
	return nil
}

func extractFrontMatter(content []byte) ([]byte, error) {
	// Allow UTF-8 BOM? For now: reject; keep deterministic.
	if len(content) < 4 {
		return nil, fmt.Errorf("missing front-matter; expected %s", HeadMatterSpec)
	}
	if !bytes.HasPrefix(content, []byte("---")) {
		return nil, fmt.Errorf("missing front-matter start delimiter; expected %s", HeadMatterSpec)
	}

	// Expect first line to be exactly '---\n' (or '---\r\n')
	startNL := bytes.IndexByte(content, '\n')
	if startNL == -1 {
		return nil, fmt.Errorf("missing newline after front-matter start delimiter")
	}
	firstLine := strings.TrimRight(string(content[:startNL]), "\r")
	if firstLine != "---" {
		return nil, fmt.Errorf("front-matter must start with a line containing only '---'")
	}

	// Find the second delimiter on its own line.
	// We search for "\n---\n" and "\n---\r\n".
	rest := content[startNL+1:]
	idx := bytes.Index(rest, []byte("\n---\n"))
	endLen := len("\n---\n")
	if idx < 0 {
		idx = bytes.Index(rest, []byte("\n---\r\n"))
		endLen = len("\n---\r\n")
	}
	if idx < 0 {
		return nil, fmt.Errorf("missing front-matter end delimiter; expected %s", HeadMatterSpec)
	}
	front := rest[:idx] // YAML only
	// sanity: ensure delimiter is its own line (we searched for newline + --- + newline)
	_ = endLen
	return front, nil
}

func validateHeadMatter(path string, hm *HeadMatter) error {
	if hm.Name == "" {
		return fmt.Errorf("%s: missing required front-matter key 'name'", path)
	}
	if hm.Version == "" {
		return fmt.Errorf("%s: missing required front-matter key 'version'", path)
	}
	if hm.Description == "" {
		return fmt.Errorf("%s: missing required front-matter key 'description'", path)
	}
	if len(hm.Requires.Bins) == 0 {
		return fmt.Errorf("%s: missing required front-matter key 'requires.bins'", path)
	}

	// Recommend matching file basename (not strictly required, but helps avoid ambiguity)
	base := strings.TrimSuffix(filepath.Base(path), ".SKILL.md")
	if base != "" && base != hm.Name {
		// Keep this as a hard error: it ensures stable lookups.
		return fmt.Errorf("%s: front-matter name '%s' must match file base name '%s'", path, hm.Name, base)
	}
	return nil
}
