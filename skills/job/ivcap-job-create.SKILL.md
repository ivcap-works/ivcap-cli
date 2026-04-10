---
name: ivcap-job-create
version: 0.1.0
description: Create an IVCAP job from a service using stdin/file input, and optionally watch/stream.
requires:
  bins: ["ivcap"]
---

# Skill: Create a job (agent-safe)

## Preconditions
- You have a valid `IVCAP_ACCESS_TOKEN` (or will pass `--access-token`).
- You know the service identifier (URN or history token).

## Best practices
- Always use `--output json`.
- Prefer `--limit` when listing services.
- Use stdin (`-f -`) to avoid writing temp files.
- Prefer explicit URNs for services (avoid `@...` history tokens when possible).

## Steps

### 1) Find a service

```bash
ivcap --output json service list --limit 10
```

### 2) Create the job

Using a file:

```bash
ivcap --output json job create <service-id> -f ./job.json --format json
```

Using stdin (preferred for agents):

```bash
cat ./job.json | ivcap --output json job create <service-id> -f - --format json
```

### 3) Watch and/or stream

Use these when you want to monitor the execution as part of the create call:

- `--watch` waits until the job finishes, then returns the final job.
- `--stream` prints job-related events as they occur, then returns the final job.

```bash
ivcap --output json job create <service-id> -f ./job.json --watch
```

```bash
ivcap --output json job create <service-id> -f ./job.json --stream
```

Tip: if you need to monitor a job later (or from another session), use the separate skill **`ivcap-job-monitor`**.
