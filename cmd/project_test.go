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
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
)

const (
	personalOnlyAccountsJSON = `{"accounts":[{"id":"urn:ivcap:account:p1","kind":"personal","name":"Me"}]}`

	workspaceAccountsJSON = `{"accounts":[` +
		`{"id":"urn:ivcap:account:w1","kind":"workspace","name":"ACME Corp"},` +
		`{"id":"urn:ivcap:account:p1","kind":"personal","name":"Me"}` +
		`]}`

	createdAccountJSON = `{"id":"urn:ivcap:account:new1","kind":"workspace","name":"My Org"}`
)

// useInput replaces the package-level stdinReader with one backed by s for
// the duration of the test, then restores the original on cleanup.
func useInput(t *testing.T, s string) {
	t.Helper()
	orig := stdinReader
	stdinReader = bufio.NewReader(strings.NewReader(s))
	t.Cleanup(func() { stdinReader = orig })
}

// ─── Non-interactive account resolution ──────────────────────────────────────
//
// In the test environment os.Stdin is not a terminal, so isInteractive()
// returns false and resolveAccountForProject always takes the non-interactive
// path regardless of other globals.

func TestResolveAccountNonInteractive_NoWorkspace(t *testing.T) {
	srv, _ := recordingServer(t, personalOnlyAccountsJSON)
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	_, err := resolveAccountForProject(context.Background(), CreateAdapter(true))
	if err == nil {
		t.Fatal("expected error when no workspace account exists")
	}
	if !strings.Contains(err.Error(), "create one") {
		t.Errorf("error should explain how to create an account, got: %v", err)
	}
}

func TestResolveAccountNonInteractive_HasWorkspaces(t *testing.T) {
	srv, _ := recordingServer(t, workspaceAccountsJSON)
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	_, err := resolveAccountForProject(context.Background(), CreateAdapter(true))
	if err == nil {
		t.Fatal("expected error in non-interactive mode even when workspaces exist")
	}
	if !strings.Contains(err.Error(), "urn:ivcap:account:w1") {
		t.Errorf("error should list workspace account URN, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ACME Corp") {
		t.Errorf("error should include account name, got: %v", err)
	}
}

// ─── Interactive account picker ───────────────────────────────────────────────

func TestSelectAccountInteractive_PicksExisting(t *testing.T) {
	useInput(t, "1\n")

	workspaces := []accountsapi.Account{
		{Id: "urn:ivcap:account:w1", Kind: "workspace", Name: "ACME Corp"},
	}
	got, err := selectAccountInteractive(context.Background(), nil, workspaces)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "urn:ivcap:account:w1" {
		t.Errorf("got %q, want urn:ivcap:account:w1", got)
	}
}

func TestSelectAccountInteractive_CreateNew(t *testing.T) {
	// "2\n" selects "Create a new workspace account"; "My Org\n" is the name.
	useInput(t, "2\nMy Org\n")

	srv, captured := recordingServer(t, createdAccountJSON)
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	workspaces := []accountsapi.Account{
		{Id: "urn:ivcap:account:w1", Kind: "workspace", Name: "ACME Corp"},
	}
	got, err := selectAccountInteractive(context.Background(), CreateAdapter(true), workspaces)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "urn:ivcap:account:new1" {
		t.Errorf("got %q, want urn:ivcap:account:new1", got)
	}
	req := findReq(captured(), http.MethodPost, "/accounts")
	if req == nil {
		t.Fatal("expected POST /accounts to be issued")
	}
	if req.body["name"] != "My Org" {
		t.Errorf("POST /accounts body: name = %v, want My Org", req.body["name"])
	}
}

func TestSelectAccountInteractive_NoExistingWorkspaces(t *testing.T) {
	useInput(t, "My New Org\n")

	const newAccJSON = `{"id":"urn:ivcap:account:new2","kind":"workspace","name":"My New Org"}`
	srv, captured := recordingServer(t, newAccJSON)
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	got, err := selectAccountInteractive(context.Background(), CreateAdapter(true), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "urn:ivcap:account:new2" {
		t.Errorf("got %q, want urn:ivcap:account:new2", got)
	}
	if findReq(captured(), http.MethodPost, "/accounts") == nil {
		t.Fatal("expected POST /accounts to be issued")
	}
}

func TestSelectAccountInteractive_InvalidInput(t *testing.T) {
	useInput(t, "99\n")

	workspaces := []accountsapi.Account{
		{Id: "urn:ivcap:account:w1", Kind: "workspace", Name: "ACME Corp"},
	}
	_, err := selectAccountInteractive(context.Background(), nil, workspaces)
	if err == nil || !strings.Contains(err.Error(), "invalid selection") {
		t.Errorf("expected 'invalid selection' error, got: %v", err)
	}
}

// ─── Workspace account creation ────────────────────────────────────────────────

func TestCreateWorkspaceAccountInteractive_EmptyName(t *testing.T) {
	useInput(t, "\n")

	_, err := createWorkspaceAccountInteractive(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "account name cannot be empty") {
		t.Errorf("expected 'account name cannot be empty', got: %v", err)
	}
}

func TestCreateWorkspaceAccountInteractive_Forbidden(t *testing.T) {
	useInput(t, "Blocked Org\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	_, err := createWorkspaceAccountInteractive(context.Background(), CreateAdapter(true))
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Errorf("expected 'not permitted' error for 403, got: %v", err)
	}
}

// TestProjectCreateFlagSkipsResolution verifies that --account-id bypasses the
// account-resolution flow entirely and no GET /accounts request is issued.
func TestProjectCreateFlagSkipsResolution(t *testing.T) {
	srv, captured := recordingServer(t, `{}`)
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	projectName = "test-project"
	projectAccountID = "urn:ivcap:account:explicit"
	t.Cleanup(func() {
		projectName = ""
		projectAccountID = ""
	})

	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		devNull.Close()
	})

	_ = createProjectCmd.RunE(createProjectCmd, nil)

	for _, r := range captured() {
		if r.method == http.MethodGet && r.path == "/accounts" {
			t.Error("GET /accounts should not be called when --account-id is supplied")
		}
	}
	req := findReq(captured(), http.MethodPost, "/projects")
	if req == nil {
		t.Fatal("expected POST /projects to be issued")
	}
	if req.body["account_id"] != "urn:ivcap:account:explicit" {
		t.Errorf("POST /projects body: account_id = %v, want urn:ivcap:account:explicit", req.body["account_id"])
	}
}

// deniedThenOK serves POST /projects with a can_create_project 403 for the
// first `denials` calls, then 200. It returns the number of calls made.
func deniedThenOK(t *testing.T, denials int) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if n <= denials {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"can_create_project on account:abc denied"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return calls }
}

func withFastGrantRetry(t *testing.T, account string) {
	t.Helper()
	oldT, oldI, oldA := grantPropagationTimeout, grantPropagationInterval, accountCreatedThisRun
	grantPropagationTimeout, grantPropagationInterval = 300*time.Millisecond, 10*time.Millisecond
	accountCreatedThisRun = account
	t.Cleanup(func() {
		grantPropagationTimeout, grantPropagationInterval, accountCreatedThisRun = oldT, oldI, oldA
	})
}

func TestCreateProjectRetriesOnFreshAccount(t *testing.T) {
	srv, calls := deniedThenOK(t, 2)
	defer srv.Close()
	setTestContext(t, srv.URL, "")
	withFastGrantRetry(t, "urn:ivcap:account:new")

	acct := "urn:ivcap:account:new"
	if _, err := createProjectAwaitingGrants(context.Background(), &accountsapi.CreateProjectPayload{Name: "p", AccountId: &acct}, CreateAdapter(true)); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls() != 3 {
		t.Errorf("calls = %d, want 3", calls())
	}
}

func TestCreateProjectGivesUpWithClearError(t *testing.T) {
	srv, _ := deniedThenOK(t, 1000)
	defer srv.Close()
	setTestContext(t, srv.URL, "")
	withFastGrantRetry(t, "urn:ivcap:account:new")

	acct := "urn:ivcap:account:new"
	_, err := createProjectAwaitingGrants(context.Background(), &accountsapi.CreateProjectPayload{Name: "p", AccountId: &acct}, CreateAdapter(true))
	if err == nil || !strings.Contains(err.Error(), "account setup is still in progress") || !strings.Contains(err.Error(), "--account-id urn:ivcap:account:new") {
		t.Errorf("expected propagation error, got %v", err)
	}
}

func TestCreateProjectDoesNotRetryExistingAccount(t *testing.T) {
	srv, calls := deniedThenOK(t, 1000)
	defer srv.Close()
	setTestContext(t, srv.URL, "")
	withFastGrantRetry(t, "urn:ivcap:account:new")

	acct := "urn:ivcap:account:other"
	if _, err := createProjectAwaitingGrants(context.Background(), &accountsapi.CreateProjectPayload{Name: "p", AccountId: &acct}, CreateAdapter(true)); err == nil {
		t.Fatal("expected the denial to be returned")
	}
	if calls() != 1 {
		t.Errorf("calls = %d, want 1 (no retry for an account we didn't just create)", calls())
	}
}

func TestResolveProjectName(t *testing.T) {
	if got, err := resolveProjectName("given", true); err != nil || got != "given" {
		t.Errorf("flag value: %q, %v", got, err)
	}

	useInput(t, "  typed  \n")
	if got, err := resolveProjectName("", true); err != nil || got != "typed" {
		t.Errorf("prompted: %q, %v", got, err)
	}

	useInput(t, "\n")
	if _, err := resolveProjectName("", true); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("empty answer: %v", err)
	}

	if _, err := resolveProjectName("", false); err == nil || !strings.Contains(err.Error(), "--name") {
		t.Errorf("non-interactive: %v", err)
	}
}

func TestSetCurrentProjectSavesNames(t *testing.T) {
	srv, _ := recordingServer(t, `{"id":"urn:ivcap:account:a1","kind":"workspace","name":"ACME Corp"}`)
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	ctxt := GetActiveContext()
	p := &accountsapi.Project{Id: "urn:ivcap:project:p1", Name: "Apollo", AccountId: "urn:ivcap:account:a1"}
	silentOrig := silent
	silent = true
	defer func() { silent = silentOrig }()
	if err := setCurrentProject(ctxt, p); err != nil {
		t.Fatal(err)
	}

	got := GetActiveContext()
	if got.CurrentProjectName != "Apollo" || got.CurrentAccountName != "ACME Corp" {
		t.Errorf("saved names = %q / %q, want Apollo / ACME Corp", got.CurrentProjectName, got.CurrentAccountName)
	}
}
