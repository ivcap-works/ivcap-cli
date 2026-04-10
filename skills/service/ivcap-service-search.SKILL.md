---
name: ivcap-service-search
version: 0.1.0
description: Search for services by keyword.
requires:
  bins: ["ivcap"]
---

# Skill: Search services (agent-safe)

## Best practices
- Always use `--output json` (global flag).
- Always use `--limit` for listing/searching.

## Examples

Search for services by keyword:

```bash
ivcap --output json service search gene --limit 10
```
