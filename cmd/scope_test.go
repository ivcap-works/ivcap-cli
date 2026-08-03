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

package cmd

import "testing"

func TestShouldSendProjectHeader(t *testing.T) {
	const proj = "urn:ivcap:project:11111111-1111-1111-1111-111111111111"
	cases := []struct {
		name           string
		requiresAuth   bool
		currentProject string
		identityScoped bool
		want           bool
	}{
		{"authed, project set, ambient -> send", true, proj, false, true},
		{"authed, project set, identity-scoped -> omit", true, proj, true, false},
		{"authed, no project -> omit", true, "", false, false},
		{"unauthenticated -> omit even with project", false, proj, false, false},
		{"identity-scoped without project -> omit", true, "", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldSendProjectHeader(c.requiresAuth, c.currentProject, c.identityScoped); got != c.want {
				t.Errorf("shouldSendProjectHeader(%v, %q, %v) = %v, want %v",
					c.requiresAuth, c.currentProject, c.identityScoped, got, c.want)
			}
		})
	}
}
