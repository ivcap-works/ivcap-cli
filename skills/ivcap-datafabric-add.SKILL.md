---
name: ivcap-datafabric-add
version: 0.1.0
description: Create (upload) an aspect into the Data Fabric from a local file or stdin.
requires:
  bins: ["ivcap"]
---

# Skill: Upload an aspect (Data Fabric add) (agent-safe)

Use this when you want to create a new **aspect** (a Data Fabric record) attached to an entity URN.

## Preconditions
- You know the **entity URN** you are attaching the aspect to.
- You know the **schema URN** for the aspect you’re adding (or you are using the command’s defaults).

## Best practices
- Prefer `--output json`.
- Prefer stdin (`-f -`) to avoid writing temp files.
- Keep schemas explicit (`-s <schema-urn>`) so the created record is deterministic.

## Examples

### 1) Add an aspect from a local file

```bash
ivcap --output json datafabric add <entity-urn> -s <schema-urn> -f ./aspect.json --format json
```

### 2) Add an aspect via stdin (agent-friendly)

```bash
cat ./aspect.json | ivcap --output json datafabric add <entity-urn> -s <schema-urn> -f - --format json
```

### 3) Verify by fetching the created aspect

The `datafabric add` result should include the created aspect URN. Then:

```bash
ivcap --output json datafabric get <aspect-urn>
```
