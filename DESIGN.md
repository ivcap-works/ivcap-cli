# Design Notes

Design notes for **ivcap-cli**.

This document explains how the codebase is structured, how the CLI maps to the IVCAP API, and the conventions to follow when adding new commands—especially with automation / AI-agent usage in mind.

## Goals

- Provide a small, predictable CLI wrapper around an IVCAP deployment.
- Keep a human-friendly interactive UX **and** support automation/agents via stable, machine-readable output.
- Make common workflows discoverable and version-matched to the installed CLI.

Non-goals:

- This is not a full client SDK for all API endpoints.
- This is not a fully “self-describing” CLI yet (there is no dedicated `schema`/`describe` command at time of writing).

## High-level architecture

The CLI is built with [Cobra](https://github.com/spf13/cobra) and organized into a few key areas:

- `cmd/` — Cobra command tree and command implementations.
- `pkg/` — API wrappers and shared library code used by commands.
- `skills/` — agent “skill docs” embedded into the CLI binary for offline, version-matched access.
- `doc/` — generated CLI reference docs (Markdown + manpages) produced from the Cobra command tree.

### Request flow

Most commands follow this flow:

1. Parse flags/args (Cobra).
2. Build an IVCAP adapter (`cmd.CreateAdapter(...)`).
3. Call an API wrapper in `pkg/` (often returning an adapter payload).
4. Print output:
   - `--output json|yaml` => machine-readable output via `pkg/adapter.ReplyPrinter`.
   - default => human-friendly tables and summaries.

## Core concepts and cross-cutting concerns

### Global flags

Global flags are defined on the root command in `cmd/root.go` and inherited by all subcommands.

Common ones:

- `--context <name>`: select which deployment context to use.
- `--access-token <token>`: override auth token.
- `--timeout <seconds>`: request timeout.
- `--output json|yaml`: machine-readable output.
- `--silent`: suppress progress output.
- `--no-history`: disable history token creation and resolution.

### Config and contexts

Contexts are stored under the OS user config directory in a folder named `ivcap-cli` (see `cmd/common.go`).

- Config file: `config.yaml`
- History file: `history.yaml`

The active context determines the base URL (and optional Host header) used by the HTTP adapter.

### Authentication

Most API calls require auth. The adapter is configured to attach a bearer token when `CreateAdapter(true)` is used.

Tokens are sourced in priority order:

1. `--access-token`
2. environment variable `IVCAP_ACCESS_TOKEN`
3. cached token in the active context

For automation/agents, prefer headless auth via env var or `--access-token`.

### Output formats

`--output json|yaml` is the primary mechanism for stable, machine-readable output.

Command implementations typically switch on `outputFormat`:

```go
switch outputFormat {
case "json", "yaml":
  return adapter.ReplyPrinter(payload, outputFormat == "yaml")
default:
  // human-friendly printing
}
```

When adding new commands, ensure JSON/YAML output paths exist and are consistent.

### History tokens (`@1`, `@2`, ...)

For human convenience, many list commands display IDs as history tokens like `@1`. Tokens are persisted to `history.yaml`.

- `MakeHistory(...)` creates tokens while printing.
- `GetHistory(...)` resolves tokens back to IDs when used as inputs.

Agents should generally avoid history tokens and prefer explicit URNs/IDs. The global `--no-history` flag disables history resolution/creation.

### Generated reference docs (`doc/`)

The repo includes generated documentation under `doc/`.

Docs are generated from the Cobra tree by calling `cmd.CreateDoc()` (see `cmd/root.go`) via the small program in `doc/create-docs.go`.

The generator also normalizes away “date-only” diffs so committed docs don’t churn.

## How to add a new command

This section describes the typical steps to implement a new CLI command in the existing style.

### 1) Choose where it fits in the command tree

Most functionality is grouped under a top-level noun:

- `ivcap service ...`
- `ivcap job ...`
- `ivcap artifact ...`
- `ivcap package ...`

If you are adding a new domain area, create a new `cmd/<domain>.go` and attach a new top-level Cobra command to `rootCmd` in that file’s `init()`.

If you are adding a subcommand to an existing domain, update the corresponding file (for example `cmd/service.go`).

### 2) Implement the Cobra command

Follow existing patterns:

- Define the command as a `*cobra.Command` value.
- Add it in `init()` using `parentCmd.AddCommand(childCmd)`.
- Add flags via helpers in `cmd/common.go` when possible.

Examples of common flag helpers:

- Listing: `addListFlags(cmd)` (adds `--limit`, `--page`, `--filter`, ...)
- Input file: `addFileFlag(cmd, "...")` and optionally `addInputFormatFlag(cmd)`

### 3) Build requests and call into `pkg/`

Prefer to keep HTTP and JSON/YAML handling in `pkg/` and `pkg/adapter`, with command code focusing on:

- request construction
- choosing output mode
- human-friendly rendering

For list operations, use `createListRequest()` from `cmd/common.go`.

For commands that accept structured payloads, follow the established `--file ...` + `--format json|yaml` pattern and use:

- `payloadFromFile(fileName, inputFormat)`

### 4) Support machine-readable output

Every command should have a JSON/YAML output path.

- For API wrappers that return adapter payloads, use `adapter.ReplyPrinter`.
- Keep the JSON shape stable.
- Avoid printing extra banners or progress output when `--output json` is set.

### 5) Human-friendly output

If the command is commonly used by humans, also provide a default output mode:

- Tables (`go-pretty/table`) for list views.
- A compact key/value view for `get` commands.

Keep names/description columns to reasonable widths (see `MAX_NAME_COL_LEN` and `WrapSoftSoft`).

### 6) Add / update generated docs

After adding or changing commands, regenerate the `doc/` reference output.

From the repo root:

```bash
go run ./doc/create-docs.go
```

This produces `doc/ivcap_*.md` and `doc/ivcap-*.3` files.

### 7) Add tests (where practical)

There are existing command tests in `cmd/*_test.go`.

Common test patterns:

- Focus on pure functions/helpers where possible.
- For command wiring, prefer calling the underlying helper functions (or extracting logic into helpers) rather than snapshotting full CLI output.

### 8) Keep agent usage in mind

When introducing a new command, ensure it is usable via:

- `--output json` for deterministic parsing.
- non-interactive auth (`IVCAP_ACCESS_TOKEN` / `--access-token`).
- safe, explicit identifiers (URNs) rather than history tokens.

See [Agent support](#agent-support) for repo-wide conventions.

## Agent support

This CLI is used both by humans and automation/AI agents. The agent-facing design centers on **predictable output** and **safe defaults**.

### Agent operating rules and guidance

Two repo documents capture operational guidance:

- `AGENTS.md`: high-priority guardrails (JSON output, headless auth, mutation confirmation, paging/limits, avoid history tokens, safe file paths).
- `skills/CONTEXT.md`: practical examples for safe automation usage.

The version-matched guidance is also served from the CLI:

```bash
ivcap --agent-context
```

If you are adding a new mutating command, ensure it can be used safely following those rules (for example: avoid hidden prompts, keep output parseable, and consider adding a future `--dry-run` mode).

### Machine-readable output (JSON/YAML)

Agent usage should prefer:

```bash
ivcap --output json ...
```

This is a global convention across commands.

### Embedded “skills” (offline, version-matched)

The CLI embeds selected workflow docs so agents can retrieve them without network access.

- Repo sources live under `skills/*.SKILL.md`.
- At build time they are embedded (see `skills/assets.go`).
- At runtime they are accessible via:
  - `ivcap skills list`
  - `ivcap skills show <skill-name>`

The `skills` commands are implemented in `cmd/skills.go` and use parsing/validation logic from `pkg/skillsdoc`.

Skill docs:

- MUST start with YAML front-matter delimited by `---`.
- MUST include required keys (`name`, `version`, `description`, `requires.bins`).
- MUST have `name` matching the file base name (enforced in `pkg/skillsdoc`).

### MCP surface

The CLI can act as an MCP server:

```bash
ivcap mcp           # stdio mode
ivcap mcp --port 8088  # SSE mode
```

Implementation notes:

- `cmd/mcp.go` lists “tool” aspects from the platform (by schema prefix) and registers them with an MCP server.
- Tool execution calls the backing IVCAP service and returns JSON results.

When adding agent-centric features, consider whether they belong in:

- the regular CLI surface (subcommands)
- embedded skills
- MCP (as tools exposed to an agent runtime)

### Command design checklist (agent-friendly)

When implementing new commands, aim for:

- Deterministic JSON/YAML output (`--output json|yaml`).
- Non-interactive flows by default (avoid browser redirects unless explicitly requested).
- Pagination controls (`--limit`, `--page`) for list commands.
- Avoid “magic” identifiers; accept explicit URNs/IDs.
- Safe file operations:
  - prefer local working directory
  - avoid path traversal patterns

## Appendix: repo map

- `cmd/root.go`: root command, global flags, adapter creation, doc generation.
- `cmd/common.go`: shared flags, config/history helpers, list request builder.
- `pkg/adapter/*`: payloads, printing helpers, transport.
- `pkg/*`: API operations called by commands.
- `skills/`: embedded skill docs; `ivcap skills ...` reads these at runtime.
- `doc/`: generated CLI docs.
