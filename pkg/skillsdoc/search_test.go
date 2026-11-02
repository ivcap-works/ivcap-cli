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

package skillsdoc_test

import (
	"reflect"
	"testing"

	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
)

func fakeDoc(name, desc, body string) *skillsdoc.SkillDoc {
	return &skillsdoc.SkillDoc{
		HeadMatter: skillsdoc.HeadMatter{Name: name, Description: desc},
		Content:    "---\nname: " + name + "\nversion: 0.1.0\ndescription: " + desc + "\nrequires:\n  bins: [ivcap]\n---\n" + body,
	}
}

func TestTokenize(t *testing.T) {
	got := skillsdoc.Tokenize("Create an IVCAP-job, v0.1!")
	want := []string{"create", "an", "ivcap", "job", "v0", "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSearchRanksNameAndDescriptionOverBody(t *testing.T) {
	docs := []*skillsdoc.SkillDoc{
		fakeDoc("alpha", "something unrelated", "mentions widgets once"),
		fakeDoc("widgets", "work with things", "body text"),
		fakeDoc("gamma", "manage widgets", "body text"),
	}
	res := skillsdoc.Search(docs, "widgets")
	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}
	order := []string{res[0].Doc.Name, res[1].Doc.Name, res[2].Doc.Name}
	want := []string{"widgets", "gamma", "alpha"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("got order %v, want %v", order, want)
	}
}

func TestSearchIgnoresFrontMatterKeysInBody(t *testing.T) {
	// "requires" / "bins" appear in every doc's front-matter only; they must
	// not be searchable via the body.
	docs := []*skillsdoc.SkillDoc{fakeDoc("alpha", "one", "two")}
	if res := skillsdoc.Search(docs, "requires bins"); len(res) != 0 {
		t.Fatalf("expected no results, got %d", len(res))
	}
}

func TestSearchNoMatchAndEmptyQuery(t *testing.T) {
	docs := []*skillsdoc.SkillDoc{fakeDoc("alpha", "one", "two")}
	for _, q := range []string{"", "   ", "---", "nonexistentterm"} {
		if res := skillsdoc.Search(docs, q); len(res) != 0 {
			t.Fatalf("query %q: expected no results, got %d", q, len(res))
		}
	}
	if res := skillsdoc.Search(nil, "alpha"); len(res) != 0 {
		t.Fatalf("expected no results for empty corpus")
	}
}

func TestSearchTiesAreBrokenByName(t *testing.T) {
	docs := []*skillsdoc.SkillDoc{
		fakeDoc("bbb", "same", "same"),
		fakeDoc("aaa", "same", "same"),
	}
	res := skillsdoc.Search(docs, "same")
	if len(res) != 2 || res[0].Doc.Name != "aaa" || res[1].Doc.Name != "bbb" {
		t.Fatalf("unexpected order: %+v", res)
	}
}

// TestSearchEmbeddedSkills pins the top hit for a few natural-language queries
// against the real embedded skill set. If a skill is renamed or its text is
// reworked, update the expectations deliberately.
func TestSearchEmbeddedSkills(t *testing.T) {
	docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
	if err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]string{
		"upload a file":           "ivcap-artifact-upload",
		"download artifact":       "ivcap-artifact-download",
		"monitor job":             "ivcap-job-monitor",
		"list services":           "ivcap-service-list",
		"search services":         "ivcap-service-search",
		"retract":                 "ivcap-datafabric-retract",
		"debug nextflow pipeline": "nextflow-debugging",
	} {
		res := skillsdoc.Search(docs, query)
		if len(res) == 0 {
			t.Errorf("query %q: no results", query)
			continue
		}
		if got := res[0].Doc.Name; got != want {
			t.Errorf("query %q: top hit %q, want %q", query, got, want)
		}
	}
}
