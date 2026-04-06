---
name: ivcap-artifact-download
version: 0.1.0
description: Download an artifact’s content to a safe local path.
requires:
  bins: ["ivcap"]
---

# Skill: Download an artifact (agent-safe)

## Best practices
- Always use `--output json` for any preceding `get/list` calls.
- Prefer **explicit URNs** (`urn:ivcap:artifact:...`) copied from JSON output.
- Avoid session-dependent `@...` history/context shortcuts; if available, use `--no-history`.
- Do **not** use `../` or absolute paths unless explicitly requested.
- Prefer writing into the current working directory.

## Steps

### 1) Find the artifact

```bash
ivcap --output json artifact list --limit 10
```

### 2) Download to a local file

```bash
ivcap artifact download <artifact-urn> -f ./artifact.bin
```

### 3) Verify metadata (optional)

```bash
ivcap --output json artifact get <artifact-urn>
```
