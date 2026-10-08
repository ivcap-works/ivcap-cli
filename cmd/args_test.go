package cmd

import (
	"fmt"
	"strings"
	"testing"
)

func TestResolveByName(t *testing.T) {
	known := map[string]string{
		"urn:ivcap:account:a1": "Me And Me",
		"urn:ivcap:account:a2": "Dup",
		"urn:ivcap:account:a3": "dup",
	}
	list := func() (map[string]string, error) { return known, nil }

	// URNs pass through without a lookup.
	failing := func() (map[string]string, error) { t.Fatal("unexpected lookup"); return nil, nil }
	if id, err := resolveByName("urn:ivcap:account:zzz", "account", failing); err != nil || id != "urn:ivcap:account:zzz" {
		t.Errorf("urn passthrough: %q, %v", id, err)
	}

	// Names match exactly, ignoring case.
	if id, err := resolveByName("me and me", "account", list); err != nil || id != "urn:ivcap:account:a1" {
		t.Errorf("name match: %q, %v", id, err)
	}

	// Unknown names are reported helpfully.
	if _, err := resolveByName("nope", "account", list); err == nil || !strings.Contains(err.Error(), `no account named "nope"`) {
		t.Errorf("unknown name: %v", err)
	}

	// Ambiguous names list the candidates.
	if _, err := resolveByName("DUP", "account", list); err == nil || !strings.Contains(err.Error(), "a2") || !strings.Contains(err.Error(), "a3") {
		t.Errorf("ambiguous name: %v", err)
	}

	// A failed lookup defers to the server.
	broken := func() (map[string]string, error) { return nil, fmt.Errorf("boom") }
	if id, err := resolveByName("x", "account", broken); err != nil || id != "x" {
		t.Errorf("lookup failure: %q, %v", id, err)
	}
}

func TestSuggestNames(t *testing.T) {
	known := map[string]string{
		"urn:1": "Me And Me Account",
		"urn:2": "ACME Corp",
		"urn:3": "Research",
		"urn:4": "Research Team",
	}
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"me-and-me-acount", []string{"urn:1"}}, // typo, separators ignored
		{"researc", []string{"urn:3", "urn:4"}}, // typo + partial, closest first
		{"me-and", []string{"urn:1"}},           // partial name
		{"zzzzzz", nil},                         // nothing close
		{"", nil},
	} {
		got := suggestNames(c.in, known)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("suggestNames(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	many := map[string]string{"1": "team a", "2": "team b", "3": "team c", "4": "team d"}
	if got := suggestNames("team", many); len(got) != maxSuggestions {
		t.Errorf("expected %d suggestions, got %v", maxSuggestions, got)
	}

	_, err := resolveByName("me-and-me-acount", "account", func() (map[string]string, error) { return known, nil })
	if err == nil || !strings.Contains(err.Error(), "Did you mean") || !strings.Contains(err.Error(), "urn:1") {
		t.Errorf("expected suggestion in error, got %v", err)
	}
}

func TestWhoamiTarget(t *testing.T) {
	old := noHistory
	defer func() { noHistory = old; history = nil }()

	noHistory, history = false, nil
	if got := whoamiTarget("Proj", "urn:ivcap:project:p1"); got != "Proj  urn:ivcap:project:p1 (@1)" {
		t.Errorf("with name: %q", got)
	}
	if got := whoamiTarget("", "urn:ivcap:account:a1"); got != "urn:ivcap:account:a1 (@2)" {
		t.Errorf("without name: %q", got)
	}

	noHistory = true
	if got := whoamiTarget("Proj", "urn:x"); got != "Proj  urn:x" {
		t.Errorf("history disabled: %q", got)
	}
}
