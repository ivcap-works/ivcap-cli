# CONTEXT.md

Agent-oriented operational guidance for **ivcap-cli**.

## IVCAP core concepts (platform mental model)

IVCAP provides a platform to run **services**. Executing a service creates a **job**.

- **Service**: a packaged capability you can execute (think “function endpoint with configuration”).
- **Job**: a single execution of a service. Jobs have status, logs, inputs, and outputs.
- **Artifact**: blob-like data consumed/produced by jobs (e.g., images, JSON, tar’d datasets). Artifacts are referenced by URN and are typically immutable.
- **Aspect**: a structured entry in the **Data Fabric** (metadata/records). Aspects can reference jobs, services, artifacts, and other domain entities.

### Provenance tracking (why aspects matter)
When you run a job, IVCAP will automatically create aspects in the Data Fabric that capture provenance relationships such as:

- which **job** used which **artifact(s)** as inputs
- which **artifact(s)** and/or **aspects** were created as outputs
- which **service** produced the results

This provenance is essential for reproducibility and auditing (e.g., “which job created this analysis aspect, and which input image did it use?”).

*Note:* additional guidance aimed specifically at **service developers** will be provided later.

## Defaults to use (agent-safe)

### Output
- Prefer **machine-readable output**:

```bash
ivcap --output json ...
```

### Auth
- Prefer headless auth:

```bash
export IVCAP_ACCESS_TOKEN="..."
ivcap --output json ...
```

Avoid browser-based `ivcap context login` unless a human confirms they can complete the flow.

### Listing resources
- Always use `--limit`.
- Use `--page` for pagination.
- Use `--filter` and `--order-by` to narrow results.

Example:

```bash
ivcap --output json service list --limit 10
```

## Resource identifiers (URNs) and history shortcuts

### URNs (preferred)
Most object references are URNs of the form:

```
urn:ivcap:{type}:{mostly-uuid}
```

Examples (shape only):
- `urn:ivcap:artifact:...`
- `urn:ivcap:service:...`
- `urn:ivcap:job:...`

Agents should prefer **explicit URNs** over any implicit or session-dependent identifiers.

### `@...` history/context shortcuts (avoid in agents)
Some commands may accept `@...` shortcuts as a form of local history/context (e.g., “the 3rd item from the last list”).

These shortcuts are:
- **session-dependent** (easy to mix up between runs)
- **non-deterministic** under concurrency or different paging/filters
- a common source of agent error

Agent guidance:
- Prefer **explicit URNs**.
- Prefer `--output json` so outputs include canonical identifiers and you can copy/paste URNs.
- If available, use `--no-history` to disable history-based resolution.

## Mutations (create/update/delete)
Treat all create/update/delete operations as **high risk**.

1. Confirm intent with the user.
2. Echo the exact command you’re going to run.
3. Prefer a future `--dry-run` mode when available.

## File paths and downloads
- Prefer writing into the current working directory.
- Avoid `../` or absolute paths unless explicitly requested.

Example:

```bash
ivcap --output json artifact download <artifact-urn> -f ./artifact.bin
```

## Using MCP
MCP avoids shell-escaping and argument parsing ambiguity.

Start MCP server (stdio):

```bash
ivcap mcp
```

Or SSE mode:

```bash
ivcap mcp --port 8088
```

## Common agent mistakes to guard against
- Hallucinated URNs / IDs (e.g. includes query params like `?fields=`).
- Incorrect history tokens (e.g. `@3` from another session).
- Path traversal in `-f/--file` arguments.
- Missing `--output json` leading to hard-to-parse output.
