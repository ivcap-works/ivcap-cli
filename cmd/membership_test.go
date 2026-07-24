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

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
)

const capsJSON = `{
	"project":[{"name":"read"},{"name":"write"},{"name":"delete"},{"name":"add_user"},{"name":"remove_user"}],
	"account":[{"name":"read"},{"name":"create_project"},{"name":"manage_members"}]
}`

type capturedReq struct {
	method   string
	path     string
	rawQuery string
	body     map[string]any
}

// recordingServer returns an httptest server that records every request. GET
// /capabilities always returns the capability vocabulary; every other request
// returns 200 with the supplied body (or "{}" if empty).
func recordingServer(t *testing.T, respBody string) (*httptest.Server, func() []capturedReq) {
	t.Helper()
	var mu sync.Mutex
	var reqs []capturedReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cr := capturedReq{method: r.Method, path: r.URL.Path, rawQuery: r.URL.RawQuery}
		if r.Body != nil {
			var b map[string]any
			if err := json.NewDecoder(r.Body).Decode(&b); err == nil {
				cr.body = b
			}
		}
		mu.Lock()
		reqs = append(reqs, cr)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/capabilities" {
			_, _ = w.Write([]byte(capsJSON))
			return
		}
		if respBody == "" {
			respBody = "{}"
		}
		_, _ = w.Write([]byte(respBody))
	}))
	return srv, func() []capturedReq {
		mu.Lock()
		defer mu.Unlock()
		out := make([]capturedReq, len(reqs))
		copy(out, reqs)
		return out
	}
}

// resetMembershipFlags clears the shared command flag globals between tests.
func resetMembershipFlags() {
	projPrincipalUser = ""
	projPrincipalService = ""
	projCapabilities = nil
	projInviteEmail = ""
	acctPrincipalUser = ""
	acctCapabilities = nil
	acctInviteEmail = ""
}

func findReq(reqs []capturedReq, method, pathContains string) *capturedReq {
	for i := range reqs {
		if reqs[i].method == method && strings.Contains(reqs[i].path, pathContains) {
			return &reqs[i]
		}
	}
	return nil
}

// ─── SDK HTTP shaping ────────────────────────────────────────────────────────

func TestSDKGrantAndRemoveShaping(t *testing.T) {
	srv, captured := recordingServer(t, "")
	defer srv.Close()
	setTestContext(t, srv.URL, "urn:ivcap:project:p1")
	ctx := context.Background()

	// Project grant — POST with principal fields + capabilities.
	if _, err := sdk.GrantProjectRaw(ctx, "P1",
		&accountsapi.AddProjectGrantPayload2{PrincipalKind: "user", PrincipalId: "U1", Capabilities: []string{"read", "write"}},
		CreateAdapter(true), logger); err != nil {
		t.Fatalf("GrantProjectRaw: %v", err)
	}
	// Account grant — POST with user_id + capabilities.
	if _, err := sdk.GrantAccountRaw(ctx, "A1",
		&accountsapi.AddAccountGrantPayload2{UserId: "U1", Capabilities: []string{"manage_members"}},
		CreateAdapter(true), logger); err != nil {
		t.Fatalf("GrantAccountRaw: %v", err)
	}
	// Per-capability project revoke — DELETE with principal_kind + capability query.
	if _, err := sdk.RemoveProjectGrantRaw(ctx, "P1", "U1", "user", "write", CreateAdapter(true), logger); err != nil {
		t.Fatalf("RemoveProjectGrantRaw: %v", err)
	}
	// Project member removal — DELETE members path with principal_kind query.
	if _, err := sdk.RemoveProjectMemberRaw(ctx, "P1", "SVC1", "service", CreateAdapter(true), logger); err != nil {
		t.Fatalf("RemoveProjectMemberRaw: %v", err)
	}
	// Account per-capability revoke — DELETE with capability query.
	if _, err := sdk.RemoveAccountGrantRaw(ctx, "A1", "U1", "manage_members", CreateAdapter(true), logger); err != nil {
		t.Fatalf("RemoveAccountGrantRaw: %v", err)
	}
	// Account member removal — DELETE members path.
	if _, err := sdk.RemoveAccountMemberRaw(ctx, "A1", "U1", CreateAdapter(true), logger); err != nil {
		t.Fatalf("RemoveAccountMemberRaw: %v", err)
	}
	// Invitation revoke — DELETE by id.
	if _, err := sdk.RevokeInvitationRaw(ctx, "INV1", CreateAdapter(true), logger); err != nil {
		t.Fatalf("RevokeInvitationRaw: %v", err)
	}

	reqs := captured()

	if r := findReq(reqs, http.MethodPost, "/projects/P1/grants"); r == nil {
		t.Error("no POST to /projects/P1/grants")
	} else {
		if r.body["principal_kind"] != "user" || r.body["principal_id"] != "U1" {
			t.Errorf("project grant body missing principal fields: %v", r.body)
		}
		if caps, ok := r.body["capabilities"].([]any); !ok || len(caps) != 2 {
			t.Errorf("project grant capabilities wrong: %v", r.body["capabilities"])
		}
	}
	if r := findReq(reqs, http.MethodPost, "/accounts/A1/grants"); r == nil {
		t.Error("no POST to /accounts/A1/grants")
	} else if r.body["user_id"] != "U1" {
		t.Errorf("account grant body missing user_id: %v", r.body)
	}
	if r := findReq(reqs, http.MethodDelete, "/projects/P1/grants/U1"); r == nil {
		t.Error("no DELETE to /projects/P1/grants/U1")
	} else if r.rawQuery != "capability=write&principal_kind=user" && r.rawQuery != "principal_kind=user&capability=write" {
		t.Errorf("remove-project-grant query = %q", r.rawQuery)
	}
	if r := findReq(reqs, http.MethodDelete, "/projects/P1/members/SVC1"); r == nil {
		t.Error("no DELETE to /projects/P1/members/SVC1")
	} else if !strings.Contains(r.rawQuery, "principal_kind=service") {
		t.Errorf("remove-project-member query = %q", r.rawQuery)
	}
	if r := findReq(reqs, http.MethodDelete, "/accounts/A1/grants/U1"); r == nil {
		t.Error("no DELETE to /accounts/A1/grants/U1")
	} else if !strings.Contains(r.rawQuery, "capability=manage_members") {
		t.Errorf("remove-account-grant query = %q", r.rawQuery)
	}
	if findReq(reqs, http.MethodDelete, "/accounts/A1/members/U1") == nil {
		t.Error("no DELETE to /accounts/A1/members/U1")
	}
	if findReq(reqs, http.MethodDelete, "/invitations/INV1") == nil {
		t.Error("no DELETE to /invitations/INV1")
	}
}

func TestSDKListMembersDecode(t *testing.T) {
	body := `{"members":[{"user_id":"U1","kind":"user","display_name":"Ada","email":"ada@x.io","capabilities":["read","write"]}]}`
	srv, _ := recordingServer(t, body)
	defer srv.Close()
	setTestContext(t, srv.URL, "urn:ivcap:project:p1")

	res, err := sdk.ListProjectMembersRaw(context.Background(), "P1", CreateAdapter(true), logger)
	if err != nil {
		t.Fatalf("ListProjectMembersRaw: %v", err)
	}
	var list accountsapi.ListMembersResult
	if err := res.AsType(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Members) != 1 || list.Members[0].UserId != "U1" || len(list.Members[0].Capabilities) != 2 {
		t.Errorf("unexpected members: %+v", list.Members)
	}
}

// ─── Command-helper logic ────────────────────────────────────────────────────

func TestPrincipalFromFlags(t *testing.T) {
	cases := []struct {
		name, user, service string
		wantKind, wantID    string
		wantErr             bool
	}{
		{name: "user", user: "urn:u", wantKind: "user", wantID: "urn:u"},
		{name: "service", service: "urn:s", wantKind: "service", wantID: "urn:s"},
		{name: "both", user: "urn:u", service: "urn:s", wantErr: true},
		{name: "neither", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetMembershipFlags()
			projPrincipalUser = c.user
			projPrincipalService = c.service
			kind, id, err := principalFromFlags()
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != c.wantKind || id != c.wantID {
				t.Errorf("got (%q,%q), want (%q,%q)", kind, id, c.wantKind, c.wantID)
			}
		})
	}
}

// TestRevokeCapabilityLoops verifies revoke-capability issues one DELETE per
// capability.
func TestRevokeCapabilityLoops(t *testing.T) {
	srv, captured := recordingServer(t, "")
	defer srv.Close()
	setTestContext(t, srv.URL, "urn:ivcap:project:p1")
	resetMembershipFlags()
	projPrincipalUser = "U1"
	projCapabilities = []string{"read", "write"}

	if err := runProjectRevokeCapability(revokeProjectCapabilityCmd, []string{"P1"}); err != nil {
		t.Fatalf("runProjectRevokeCapability: %v", err)
	}
	n := 0
	for _, r := range captured() {
		if r.method == http.MethodDelete && strings.Contains(r.path, "/projects/P1/grants/U1") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("expected 2 DELETE requests (one per capability), got %d", n)
	}
}

// TestGrantValidatesCapabilities verifies an unknown capability is rejected before
// any grant request is sent.
func TestGrantValidatesCapabilities(t *testing.T) {
	srv, captured := recordingServer(t, "")
	defer srv.Close()
	setTestContext(t, srv.URL, "urn:ivcap:project:p1")
	resetMembershipFlags()
	projPrincipalUser = "U1"
	projCapabilities = []string{"bogus"}

	err := runProjectGrant(grantProjectCmd, []string{"P1"})
	if err == nil || !strings.Contains(err.Error(), "unknown capabilit") {
		t.Fatalf("expected unknown-capability error, got: %v", err)
	}
	for _, r := range captured() {
		if r.method == http.MethodPost && strings.Contains(r.path, "/grants") {
			t.Error("no grant request should be sent when validation fails")
		}
	}
}

// TestAccountHasNoServiceFlag guards the user-only nature of account grants.
func TestAccountHasNoServiceFlag(t *testing.T) {
	if grantAccountCmd.Flags().Lookup("service") != nil {
		t.Error("account grant must not expose a --service flag (account grants are user-only)")
	}
	if grantAccountCmd.Flags().Lookup("user") == nil {
		t.Error("account grant must expose a --user flag")
	}
}
