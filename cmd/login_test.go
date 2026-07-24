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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// setTestContext points the config at a temp dir with a single active context at
// baseURL, and resets the token-related globals so each test is hermetic.
func setTestContext(t *testing.T, baseURL, currentProject string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// reset process-global auth state that persists across CreateAdapter calls
	accessToken = ""
	accessTokenF = ""
	accessTokenProvided = false
	contextName = ""
	timeout = 30

	cfg := &Config{
		Version:       "v1",
		ActiveContext: "test",
		Contexts: []Context{{
			ApiVersion:        1,
			Name:              "test",
			URL:               baseURL,
			CurrentProject:    currentProject,
			AccessToken:       "test-token",
			AccessTokenExpiry: time.Now().Add(time.Hour),
		}},
	}
	WriteConfigFile(cfg)
}

func discoveryJSON(base string) string {
	return `{
		"issuer": "` + base + `",
		"token_endpoint": "` + base + `/oauth2/token",
		"device_authorization_endpoint": "` + base + `/oauth2/device/auth",
		"jwks_uri": "` + base + `/.well-known/jwks.json"
	}`
}

func TestFetchOIDCDiscovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == OIDC_DISCOVERY_PATH {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(discoveryJSON("https://id.example.com")))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	d, err := fetchOIDCDiscovery(GetActiveContext())
	if err != nil {
		t.Fatalf("fetchOIDCDiscovery: %v", err)
	}
	if d.TokenEndpoint != "https://id.example.com/oauth2/token" {
		t.Errorf("token_endpoint = %q", d.TokenEndpoint)
	}
	if d.DeviceAuthorizationEndpoint != "https://id.example.com/oauth2/device/auth" {
		t.Errorf("device_authorization_endpoint = %q", d.DeviceAuthorizationEndpoint)
	}
	if d.Issuer != "https://id.example.com" {
		t.Errorf("issuer = %q", d.Issuer)
	}
}

func TestFetchOIDCDiscoveryUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	if _, err := fetchOIDCDiscovery(GetActiveContext()); err == nil {
		t.Fatal("expected error when discovery is unavailable, got nil")
	}
}

func TestResolveAuthProviderOIDC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == OIDC_DISCOVERY_PATH {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(discoveryJSON("https://id.example.com")))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setTestContext(t, srv.URL, "")

	ap, mode := resolveAuthProvider(GetActiveContext())
	if mode != authModeOIDC {
		t.Fatalf("mode = %q, want %q", mode, authModeOIDC)
	}
	if ap.ClientID != IVCAP_CLI_CLIENT_ID {
		t.Errorf("client id = %q, want %q", ap.ClientID, IVCAP_CLI_CLIENT_ID)
	}
	if ap.Audience != "" {
		t.Errorf("audience should be empty for OIDC, got %q", ap.Audience)
	}
	if ap.TokenURL == "" || ap.CodeURL == "" {
		t.Errorf("expected token/code URLs from discovery, got token=%q code=%q", ap.TokenURL, ap.CodeURL)
	}
}

func TestRequestDeviceCodeAudienceOmission(t *testing.T) {
	for _, tc := range []struct {
		name       string
		audience   string
		wantHasAud bool
	}{
		{"omitted when empty", "", false},
		{"present when set", "https://api.example.com", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hasAud bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				_, hasAud = r.PostForm["audience"]
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"uc","verification_uri_complete":"http://x"}`))
			}))
			defer srv.Close()
			setTestContext(t, srv.URL, "")

			ap := &AuthProvider{
				CodeURL:  srv.URL + "/oauth2/device/auth",
				ClientID: IVCAP_CLI_CLIENT_ID,
				Audience: tc.audience,
				scopes:   "openid profile email offline_access",
			}
			_ = requestDeviceCode(ap)
			if hasAud != tc.wantHasAud {
				t.Errorf("audience present = %v, want %v", hasAud, tc.wantHasAud)
			}
		})
	}
}

func TestCreateAdapterProjectHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(PROJECT_HEADER)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// Authenticated adapter with a current project → header present.
	setTestContext(t, srv.URL, "urn:ivcap:project:p1")
	got = ""
	if _, err := (*CreateAdapter(true)).Get(context.Background(), "/x", logger); err != nil {
		t.Fatalf("authed get: %v", err)
	}
	if got != "urn:ivcap:project:p1" {
		t.Errorf("authed request %s = %q, want the project urn", PROJECT_HEADER, got)
	}

	// Unauthenticated adapter → no project header (discovery/token calls).
	got = ""
	if _, err := (*CreateAdapter(false)).Get(context.Background(), "/x", logger); err != nil {
		t.Fatalf("unauth get: %v", err)
	}
	if got != "" {
		t.Errorf("unauth request should not carry %s, got %q", PROJECT_HEADER, got)
	}

	// Authenticated adapter but no current project → no header.
	setTestContext(t, srv.URL, "")
	got = ""
	if _, err := (*CreateAdapter(true)).Get(context.Background(), "/x", logger); err != nil {
		t.Fatalf("authed get (no project): %v", err)
	}
	if got != "" {
		t.Errorf("no-project request should not carry %s, got %q", PROJECT_HEADER, got)
	}
}
