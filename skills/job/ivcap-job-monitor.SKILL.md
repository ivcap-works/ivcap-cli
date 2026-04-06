---
name: ivcap-job-monitor
version: 0.1.0
description: Monitor job status and stream events.
requires:
  bins: ["ivcap"]
---

# Skill: Monitor a job (agent-safe)

## Best practices
- Always use `--output json`.
- Prefer explicit URNs.

## Examples

Get job status:

```bash
ivcap --output json job get <job-urn>
```

Stream job events:

```bash
ivcap --output json job get <job-urn> --stream
```
