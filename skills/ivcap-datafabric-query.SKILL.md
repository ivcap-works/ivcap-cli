---
name: ivcap-datafabric-query
version: 0.1.0
description: Query Data Fabric aspects by entity, schema prefix and time with agent-safe paging/filters.
requires:
  bins: ["ivcap"]
---

# Skill: Query aspects (Data Fabric query) (agent-safe)

Use `ivcap datafabric query` to find aspects by **entity**, **schema prefix**, and optionally a point-in-time.

## Best practices
- Always use `--output json`.
- Always use `--limit` and paginate with `--page`.
- Prefer narrowing results with `--entity` and `--schema-prefix` (or `--schema`).
- Use `--include-content` only when necessary (it can be large/noisy).

## Examples

### 1) Query aspects for an entity

```bash
ivcap --output json datafabric query --entity <entity-urn> --limit 10
```

### 2) Query by entity + schema prefix

```bash
ivcap --output json datafabric query --entity <entity-urn> --schema-prefix urn:ivcap:schema:job. --limit 10
```

### 3) Query “as of” a time in the past

```bash
ivcap --output json datafabric query --entity <entity-urn> --at-time "2026-03-01T00:00:00Z" --limit 10
```

### 4) Query and include aspect content

```bash
ivcap --output json datafabric query --entity <entity-urn> --schema-prefix <schema-prefix> --include-content --limit 10
```

### 5) Convenience: if exactly one match, fetch it immediately

```bash
ivcap --output json datafabric query --entity <entity-urn> --schema-prefix <schema-prefix> --get-if-one
```
