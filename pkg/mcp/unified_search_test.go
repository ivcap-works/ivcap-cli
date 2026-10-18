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

package mcp

import "testing"

func TestSearchSkillsLocalRanksByRelevance(t *testing.T) {
	res, err := searchSkillsLocal("upload a file", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(res))
	}
	if res[0].Name != "ivcap-artifact-upload" {
		t.Fatalf("top hit %q, want ivcap-artifact-upload", res[0].Name)
	}
	if res[0].URI == "" {
		t.Fatalf("expected a URI on the result")
	}
}

func TestSearchSkillsLocalListAllAndErrors(t *testing.T) {
	all, err := searchSkillsLocal("*", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Fatalf("expected several skills, got %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Fatalf("'*' results not sorted by name: %q before %q", all[i-1].Name, all[i].Name)
		}
	}
	if _, err := searchSkillsLocal("zzzznomatch", 5); err == nil {
		t.Fatalf("expected an error when nothing matches")
	}
	if _, err := searchSkillsLocal("   ", 5); err == nil {
		t.Fatalf("expected an error for an empty query")
	}
}
