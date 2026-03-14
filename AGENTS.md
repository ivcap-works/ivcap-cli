# AGENTS.md

This repository contains **ivcap-cli**, a command-line tool for interacting with an IVCAP deployment.

It is frequently invoked by **AI/LLM agents** (via shell or via MCP). Agents are fast and confident, but can be wrong in *different* ways than humans. Treat agent-provided inputs as potentially unsafe.

## Agent operating rules (high priority)

### Prefer machine-readable output
- For any command whose output will be consumed programmatically, **always** set:
  - `--output json` (global flag; works for all commands)

### Prefer non-interactive auth
- Prefer headless auth via environment variables:
  - `IVCAP_ACCESS_TOKEN` (or `--access-token`)
- Avoid flows that require a browser redirect unless a human explicitly confirms they are present.

### Confirm before mutations
Before running any command that **creates/updates/deletes** resources (services, secrets, queues, uploads, etc.):
- Confirm intent with the user.
- Echo back the command you plan to run.
- If/when a `--dry-run` mode exists, use it first.

### Keep responses small
When listing resources:
- Use `--limit` and paging.
- Prefer selecting only necessary fields (future: `--fields/--select`).

### Prefer explicit URNs; avoid history shortcuts
- Prefer explicit `urn:ivcap:{type}:...` identifiers copied from JSON output.
- Avoid session-dependent `@...` history/context shortcuts.
- If available, use `--no-history` to disable history-based resolution.

### Don’t trust file paths
If a command writes files locally (downloads, exports):
- Use explicit, safe output paths.
- Avoid path traversal (`../`) and avoid writing outside the working directory unless the user explicitly asks.

## Recommended agent surfaces

### MCP (preferred)
If your agent runtime supports MCP, start the server and call tools via JSON-RPC (avoids shell escaping):

```bash
ivcap mcp
```

For SSE mode:

```bash
ivcap mcp --port 8088
```

### CLI (fallback)
If invoking the CLI directly, prefer:
- `--output json`
- file/stdin-based structured input where supported (e.g. `job create -f - --format json`)

## Additional agent context
- See `skills/CONTEXT.md` (or `ivcap --agent-context`) for operational guidance and patterns.
- See `skills/` for higher-level workflow “skills” with examples.
