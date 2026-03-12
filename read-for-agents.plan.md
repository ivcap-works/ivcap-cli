 # read-for-agents.plan.md

## Goal
Incorporate agent-first CLI principles from “You Need to Rewrite Your CLI for AI Agents” into **ivcap-cli** without breaking the existing human-friendly UX.

## Key ideas from the article (condensed)
- **Machine-readable I/O first**: raw JSON payloads and deterministic output beat bespoke flags for agents.
- **Runtime schema introspection**: make the CLI its own documentation (`schema`/`describe`).
- **Context window discipline**: field masks + streaming/NDJSON pagination to limit token load.
- **Input hardening**: assume hallucinated inputs (path traversal, control chars, URL-encoded strings).
- **Ship agent skills**: structured, agent-focused guidance (skills/context files) that are *cheap and reliable* for agents to access at runtime ("a skill file is cheaper than a hallucination").
- **Multi-surface support**: CLI + MCP + extension/automation surfaces from one binary.
- **Safety rails**: `--dry-run` and response sanitization for agent-safe operations.

## Current ivcap-cli alignment
**Already present**
- All commands inherit the global `-o, --output [json|yaml]` flag for machine-readable output (see `cmd/root.go`).
- MCP server surface (`ivcap mcp`) with tool schemas pulled from aspects (see `cmd/mcp.go`).
- JSON/YAML input via `--file` and `--format` for job creation and related flows.
- `IVCAP_ACCESS_TOKEN` env var for headless auth usage.
- Some streaming behavior (e.g., `job create --stream`).
- CLI is a wrapper around the OpenAPI spec at https://github.com/ivcap-works/ivcap-core-api/blob/main/openapi3.json.

**Gaps / opportunities**
- No explicit **schema/describe** command for runtime introspection.
- Limited **raw JSON payload** support (agents must write files; no `--json`/`--params`).
- No **NDJSON / page-all** streaming for large list outputs.
- Minimal **input hardening** (paths, URNs, control chars, URL encoding).
- No **dry-run** support on mutating operations.
- No **agent skills / context docs** for agent-specific guidance.

## Plan (phased, incremental)

### Phase 0 — Agent guidance (quick wins)
**Deliverables**
- Add `AGENTS.md` at repo root and `skills/CONTEXT.md` for explicit agent rules:
  - Always use `--output json` for tool consumption.
  - Prefer `--limit`, `--page`, and minimal fields when listing.
  - Confirm with the user before any write/delete operations.
  - Use `--dry-run` once available.

#### Phase 0a — Skills as *installed* (version-matched) assets
The whole point of SKILL.md is that it's **cheap and reliable** from an agent's perspective. A link to a repo is neither.

Adopt a **two-layer approach**:
1. **Agent source of truth (primary):** embed the SKILL.md files as assets in the *distributed CLI* (binary or install package), version-matched to the CLI itself.
2. **Human discoverability (secondary):** keep the same files in the repo for browsing/search, but treat them as informational only.

If the repo and installed contents ever diverge, the **installed version wins**.

**Deliverables**
- Keep a `skills/` folder in the repo with YAML-frontmatter skill docs for common workflows:
  - `ivcap-job-create`, `ivcap-artifact-download`, `ivcap-service-list`.
- Embed those files into the CLI release artifact (preferred: Go `embed`; alternatively: packaged alongside the binary with a fixed, versioned install path).
- Expose them via a CLI surface so agents can pull them into context **offline** with **no network call**:
  - `ivcap skills list` (enumerate available skills)
  - `ivcap skills show <skill-name>` (print the exact SKILL.md content)

This mirrors the article's approach for `schema`: the CLI becomes **self-describing and queryable**, not dependent on external resources.

**Output conventions**
- `ivcap skills list --output json` should return a stable JSON shape (e.g., `{ "skills": [{"name": "ivcap-job-create", "path": "...", "sha256": "..."}] }`).
- `ivcap skills show <name> --output json` should return `{ "name": "...", "content": "...", "sha256": "..." }` (so agents can checksum and cache).

**Success criteria**
- An agent can do `ivcap skills list --output json` and `ivcap skills show ivcap-job-create` to retrieve version-matched skill text without any network dependency.

- Update `README.md` and `doc/ivcap_mcp.md` to reference agent guidance + MCP usage.

**Success criteria**
- Agents can load repository context and operate with explicit rules.

### Phase 1 — Machine-readable input/output upgrades
**Deliverables**
- Add `--json` or `--params` flags for commands that accept structured input (starting with `job create`, `service create/update`, `artifact create`).
- Support `--file -` + stdin for JSON to reduce file I/O friction for agents.
- Add auto-switch to JSON when stdout is non-TTY (optional, behind env var).

**Success criteria**
- Agents can call commands with inline JSON payloads and deterministic JSON output.

### Phase 2 — Schema introspection
**Deliverables**
- New `ivcap schema` (or `ivcap describe`) command to emit request/response schemas.
  - Slice the relevant endpoint and component schema definitions from the
    OpenAPI spec at https://github.com/ivcap-works/ivcap-core-api/blob/main/openapi3.json.
  - Include required scopes, input parameters, and output schema in JSON.
- Add `--describe` on key commands to print schemas inline.

#### Phase 2a — "Agent help" (machine-readable `--help`)
Human `--help` is optimized for readability, not tool consumption. Agents need a deterministic, structured equivalent.

**Suggestion: add a global `--agent-help` flag** (or `ivcap agent-help` subcommand) that returns a stable JSON document describing:

**Top-level `ivcap --agent-help --output json` should include**
- CLI identity:
  - `name`, `version`, `build`, `commit` (if available)
  - `default_output_formats` (json/yaml)
- Global conventions/guardrails (summarized; links to fuller text):
  - recommended `--output json` usage
  - non-interactive auth (`IVCAP_ACCESS_TOKEN`, `--access-token`)
  - mutation confirmation + `--dry-run` availability
- Global flags (name, type, default, description) and env-var aliases.
- Command index (so an agent can discover what exists without parsing text):
  - command `path` (e.g., `service list`), `short`, `long` (optional), and whether it is mutating.
  - supported output modes (json/yaml/ndjson) if applicable.
- Skill index integration:
  - either embed the same payload as `ivcap skills list --output json` or include a pointer (`skills_command: "ivcap skills"`).
- Schema/introspection integration:
  - include a pointer to `ivcap schema` / `--describe` (e.g., `schema_command: "ivcap schema"`).

**Per-command `--agent-help` (extending across all commands that have `--help`)**
For any command `ivcap <path> ...`, support:
- `ivcap <path> --agent-help --output json`

This should return:
- command identity: `path`, `aliases`, `short`, `long` (optional)
- arguments: positional args (name, required/optional, validation notes if known)
- flags:
  - for each flag: `name`, `shorthand`, `type`, `default`, `required` (if enforced), `description`
  - for each env var mapping (if any): include `env` hints
- I/O:
  - request payload entrypoints (`--file`, stdin via `--file -`, future `--json/--params`)
  - response shape pointer: either inline a minimal schema or a pointer to `ivcap schema` for full OpenAPI-derived schema
- examples:
  - a small set of canonical JSON-first examples (token-efficient)
- related skills:
  - list skill names relevant to this command (e.g., `ivcap-service-list`), so the agent can immediately `ivcap skills show <name>`.

**Implementation note (so this scales):**
- Build `--agent-help` by walking the Cobra command tree at runtime (flags/args/subcommands are already in-memory).
- Keep the payload versioned, e.g. `agent_help_version: 1`, so agents can rely on schema stability.
- Avoid embedding long prose; treat this as a *machine-readable index*. Detailed guidance remains in `skills/CONTEXT.md` and SKILL.md.

**Success criteria**
- Agents can discover parameters and schemas without external docs.

### Phase 3 — Context window discipline
**Deliverables**
- Add `--fields` (or `--select`) to list/get commands to filter output fields.
- Add `--page-all` + NDJSON output for paginated list commands.
- Update `skills/CONTEXT.md` with guidance on field masks and paging.

**Success criteria**
- Large list responses can be streamed/filtered to minimize token usage.

### Phase 4 — Input hardening + safety rails
**Deliverables**
- Central validation helpers (in `cmd/common.go`):
  - Reject control characters in identifiers and file names.
  - Disallow `?`, `#`, and `%` in resource IDs to prevent embedded queries/double-encoding.
  - Normalize and sandbox output paths to prevent traversal.
- Add `--dry-run` for mutating commands (validate payload without execution).
- Optional `--sanitize` output path for integrating response filtering (future-proof).

**Success criteria**
- CLI fails safely on hallucinated inputs; mutating commands can be validated.

### Phase 5 — MCP surface refinements
**Deliverables**
- Extend `ivcap mcp` to include:
  - Tool filtering (`--tools`, `--service`),
  - Introspection of tool schemas (`--list-tools`),
  - NDJSON/JSON responses by default.
- Provide sample MCP client config in `mcp-inspector.config.json` + docs.

**Success criteria**
- MCP tool list is predictable, filterable, and self-describing.

## Constraints
- Preserve existing human-friendly output for TTY by default.
- Maintain backward compatibility for existing scripts.
- Keep changes incremental, feature-flagged where needed.
- Prefer *installed* skill assets as the source of truth for agents; repo-hosted copies are secondary.

## Suggested next steps (implementation order)
1. Draft `AGENTS.md` / `skills/CONTEXT.md` + starter skill files.
2. Add `--json`/`--params` input support for job/service/artifact operations.
3. Implement `ivcap schema` and `--describe` on key commands.
4. Add `--fields` and `--page-all` with NDJSON output.
5. Add input validation helpers + `--dry-run` for mutating commands.
6. Enhance MCP surface with tool filtering and schema discovery.