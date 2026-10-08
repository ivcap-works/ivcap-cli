package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/spf13/cobra"
)

// positionalNames returns the leading positional argument names declared in a
// command's Use string, e.g. "get [flags] project_id" -> ["project_id"].
func positionalNames(cmd *cobra.Command) []string {
	fields := strings.Fields(cmd.Use)
	var names []string
	for _, f := range fields[min(1, len(fields)):] {
		if f == "[flags]" {
			continue
		}
		if strings.HasPrefix(f, "-") || strings.HasPrefix(f, "(") || strings.HasPrefix(f, "[") {
			break
		}
		names = append(names, f)
	}
	return names
}

// argsError builds a readable error for a wrong number of positional arguments.
func argsError(cmd *cobra.Command, got int, tooMany bool) error {
	names := positionalNames(cmd)
	var msg string
	switch {
	case tooMany:
		msg = fmt.Sprintf("too many arguments (got %d)", got)
	case len(names) > 0 && got < len(names):
		msg = fmt.Sprintf("missing required argument: %s", strings.Join(names[got:], " "))
	default:
		msg = "missing required argument"
	}
	return fmt.Errorf("%s\nUsage: %s\nRun '%s --help' for details", msg, cmd.UseLine(), cmd.CommandPath())
}

// exactArgs is cobra.ExactArgs with an error message naming the missing arguments.
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return argsError(cmd, len(args), len(args) > n)
		}
		return nil
	}
}

// minimumNArgs is cobra.MinimumNArgs with a readable error message.
func minimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return argsError(cmd, len(args), false)
		}
		return nil
	}
}

// optionalArg allows zero or one positional argument (the target may then
// come from the current context).
func optionalArg(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return argsError(cmd, len(args), true)
	}
	return nil
}

// projectArg resolves the project from args[0] (through the @N history), or
// falls back to the current project, telling the user which one is used.
func projectArg(args []string) (string, error) {
	return defaultedArg(args, "project", GetActiveContext().CurrentProject, resolveProjectID,
		"no project given and no current project; pass a project_id or run 'ivcap context project use'")
}

// accountArg resolves the account from args[0] (through the @N history), or
// falls back to the account of the current project.
func accountArg(args []string) (string, error) {
	return defaultedArg(args, "account", GetActiveContext().AccountID, resolveAccountID,
		"no account given and no current project to take it from; pass an account_id or run 'ivcap context project use'")
}

func defaultedArg(args []string, kind, def string, resolve func(string) (string, error), noDefaultMsg string) (string, error) {
	if len(args) > 0 {
		return resolve(args[0])
	}
	if def == "" {
		return "", fmt.Errorf("%s", noDefaultMsg)
	}
	if !silent {
		fmt.Fprintf(os.Stderr, "No %s given, using current %s %s\n", kind, kind, def)
	}
	return def, nil
}

// resolveAccountID resolves an account argument to its URN. Besides @N history
// tokens it accepts the account's display name, as shown by 'account list'.
func resolveAccountID(arg string) (string, error) {
	return resolveByName(arg, "account", func() (map[string]string, error) {
		res, err := sdk.ListAccounts(context.Background(), &sdk.ListRequest{Limit: nameLookupLimit}, GetIdentityAdapter(true), logger)
		if err != nil {
			return nil, err
		}
		m := make(map[string]string, len(res.Accounts))
		for _, a := range res.Accounts {
			m[a.Id] = a.Name
		}
		return m, nil
	})
}

// resolveProjectID is resolveAccountID for projects.
func resolveProjectID(arg string) (string, error) {
	return resolveByName(arg, "project", func() (map[string]string, error) {
		res, err := sdk.ListProjects(context.Background(), &sdk.ListRequest{Limit: nameLookupLimit}, GetIdentityAdapter(true), logger)
		if err != nil {
			return nil, err
		}
		m := make(map[string]string, len(res.Projects))
		for _, p := range res.Projects {
			m[p.Id] = p.Name
		}
		return m, nil
	})
}

const nameLookupLimit = 100

// resolveByName maps a human-friendly name to an id (via the id->name map from
// list). Anything that already looks like a URN is passed through untouched, as
// is everything when the lookup itself fails, so the server reports the real
// problem. Names match exactly, ignoring case.
func resolveByName(arg, kind string, list func() (map[string]string, error)) (string, error) {
	id := GetHistory(arg)
	if strings.HasPrefix(id, "urn:") {
		return id, nil
	}
	known, err := list()
	if err != nil {
		return id, nil
	}
	var matches []string
	for mid, name := range known {
		if strings.EqualFold(name, id) {
			matches = append(matches, mid)
		}
	}
	switch len(matches) {
	case 0:
		msg := fmt.Sprintf("no %s named %q found; use the %s URN or see 'ivcap context %s list'", kind, id, kind, kind)
		if sugg := suggestNames(id, known); len(sugg) > 0 {
			msg += "\nDid you mean:"
			for _, sid := range sugg {
				msg += fmt.Sprintf("\n  %q (%s)", known[sid], sid)
			}
		}
		return "", errors.New(msg)
	case 1:
		if !silent {
			fmt.Fprintf(os.Stderr, "Using %s %q (%s)\n", kind, id, matches[0])
		}
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("%d %ss are named %q; use one of these URNs instead:\n  %s", len(matches), kind, id, strings.Join(matches, "\n  "))
	}
}

const maxSuggestions = 3

// suggestNames returns up to maxSuggestions ids from known (id -> name) whose
// names are close to input, best first. Names are compared ignoring case and
// separators; a name is close if it is within a third of its length in edit
// distance, or if one contains the other.
func suggestNames(input string, known map[string]string) []string {
	in := normaliseName(input)
	if in == "" {
		return nil
	}
	type cand struct {
		id   string
		name string
		dist int
	}
	var cands []cand
	for id, name := range known {
		n := normaliseName(name)
		if n == "" {
			continue
		}
		d := editDistance(in, n)
		limit := max(len(in), len(n)) / 3
		if strings.Contains(n, in) || strings.Contains(in, n) {
			// containment counts as close, ranked after genuine typos
			d = min(d, limit)
		} else if d > limit {
			continue
		}
		cands = append(cands, cand{id, name, d})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		if cands[i].name != cands[j].name {
			return cands[i].name < cands[j].name
		}
		return cands[i].id < cands[j].id
	})
	var out []string
	for i := 0; i < len(cands) && i < maxSuggestions; i++ {
		out = append(out, cands[i].id)
	}
	return out
}

// normaliseName lowercases s and drops spaces, hyphens and underscores.
func normaliseName(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '_':
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
