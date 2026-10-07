package cmd

import (
	"fmt"
	"os"
	"strings"

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
	return defaultedArg(args, "project", GetActiveContext().CurrentProject,
		"no project given and no current project; pass a project_id or run 'ivcap context project use'")
}

// accountArg resolves the account from args[0] (through the @N history), or
// falls back to the account of the current project.
func accountArg(args []string) (string, error) {
	return defaultedArg(args, "account", GetActiveContext().AccountID,
		"no account given and no current project to take it from; pass an account_id or run 'ivcap context project use'")
}

func defaultedArg(args []string, kind, def, noDefaultMsg string) (string, error) {
	if len(args) > 0 {
		return GetHistory(args[0]), nil
	}
	if def == "" {
		return "", fmt.Errorf("%s", noDefaultMsg)
	}
	if !silent {
		fmt.Fprintf(os.Stderr, "No %s given, using current %s %s\n", kind, kind, def)
	}
	return def, nil
}
