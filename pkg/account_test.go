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

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
)

// TestAccountSDKPathsAndVerbs asserts each ivcap-accounts wrapper hits the
// expected HTTP method + path, guarding against drift in the SDK request builders.
func TestAccountSDKPathsAndVerbs(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	ad := adapter.RestAdapter(adapter.WithConnContext(&adapter.ConnectionCtxt{URL: srv.URL, TimeoutSec: 5}))
	adp := &ad
	logger := log.NewNop()
	ctx := context.Background()

	// Paths are asserted with the /1 gateway prefix spelled out, not composed from
	// accountsAPIPrefix — the point is to pin the bytes that go on the wire, which a
	// test built from the same constant could not do.
	cases := []struct {
		name, method, path string
		call               func() error
	}{
		{"list accounts", "GET", "/1/accounts", func() error { _, e := ListAccountsRaw(ctx, &ListRequest{}, adp, logger); return e }},
		{"read account", "GET", "/1/accounts/urn:acc", func() error { _, e := ReadAccountRaw(ctx, "urn:acc", adp, logger); return e }},
		{"create account", "POST", "/1/accounts", func() error { _, e := CreateAccountRaw(ctx, "name", adp, logger); return e }},
		{"list projects", "GET", "/1/projects", func() error { _, e := ListProjectsRaw(ctx, &ListRequest{}, adp, logger); return e }},
		{"read project", "GET", "/1/projects/urn:p", func() error { _, e := ReadProjectRaw(ctx, "urn:p", adp, logger); return e }},
		{"create project", "POST", "/1/projects", func() error {
			_, e := CreateProjectRaw(ctx, &accountsapi.CreateProjectPayload2{Name: "n"}, adp, logger)
			return e
		}},
		{"update project", "PATCH", "/1/projects/urn:p", func() error {
			_, e := UpdateProjectRaw(ctx, "urn:p", &accountsapi.UpdateProjectPayload2{Name: "n"}, adp, logger)
			return e
		}},
		{"delete project", "DELETE", "/1/projects/urn:p", func() error { _, e := DeleteProjectRaw(ctx, "urn:p", adp, logger); return e }},
		{"leave project", "POST", "/1/projects/urn:p/leave", func() error { _, e := LeaveProjectRaw(ctx, "urn:p", adp, logger); return e }},
		{"grant project", "POST", "/1/projects/urn:p/grants", func() error {
			_, e := GrantProjectRaw(ctx, "urn:p", &accountsapi.AddProjectGrantPayload2{}, adp, logger)
			return e
		}},
		{"list my invitations", "GET", "/1/invitations/mine", func() error { _, e := ListMyInvitationsRaw(ctx, adp, logger); return e }},
		{"accept invitation", "POST", "/1/invitations/i1/accept", func() error { _, e := AcceptInvitationRaw(ctx, "i1", adp, logger); return e }},
		{"decline invitation", "POST", "/1/invitations/i1/decline", func() error { _, e := DeclineInvitationRaw(ctx, "i1", adp, logger); return e }},
		{"create project invitation", "POST", "/1/projects/urn:p/invitations", func() error {
			_, e := CreateProjectInvitationRaw(ctx, "urn:p", &accountsapi.CreateInvitationPayload2{Email: "e@x"}, adp, logger)
			return e
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotMethod, gotPath = "", ""
			if err := c.call(); err != nil {
				t.Fatalf("call error: %v", err)
			}
			if gotMethod != c.method || gotPath != c.path {
				t.Errorf("got %s %s, want %s %s", gotMethod, gotPath, c.method, c.path)
			}
		})
	}
}
