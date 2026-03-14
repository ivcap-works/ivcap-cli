---
name: ivcap-service-list
version: 0.1.0
description: List and inspect services with agent-safe defaults (limit, json output).
requires:
  bins: ["ivcap"]
---

# Skill: List services (agent-safe)

## Best practices
- Always use `--output json` (global flag).
- Always use `--limit` for listing.
- Use `--filter` to narrow results.

## Examples

List services (small page):

```bash
ivcap --output json service list --limit 10
```

Filter services (example):

```bash
ivcap --output json service list --limit 10 --filter 'name~=gene'
```

Get service details:

```bash
ivcap --output json service get <service-urn-or-@token>
```
