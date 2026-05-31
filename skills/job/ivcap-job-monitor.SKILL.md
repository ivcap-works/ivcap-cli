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

Stream job events (recommended):

```bash
ivcap job events <service-id> <job-id>
```

Stream job events with limit:

```bash
ivcap job events --max-messages 10 <service-id> <job-id>
```

Resume from last event:

```bash
ivcap job events --last-event-id <event-id> <service-id> <job-id>
```

Alternative - create job with streaming:

```bash
ivcap job create <service-id> -f input.json --stream
```
