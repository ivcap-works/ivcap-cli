---
name: ivcap-job-monitor
version: 0.1.0
description: Monitor a job after creation using job get (polling) and job create --watch/--stream.
requires:
  bins: ["ivcap"]
---

# Skill: Monitor a job (agent-safe)

This skill focuses on **monitoring** a job *after you have a job id/URN*, and on the two create-time monitoring modes.

## Best practices
- Use `--output json` so you can reliably extract fields.
- Prefer explicit URNs/IDs returned by JSON output.
- Avoid `@...` history tokens unless you are in the same session and are certain they refer to the right job.

## Option A: Monitor during creation (one-shot)

### Wait until finished (`--watch`)

```bash
ivcap --output json job create <service-id> -f ./job.json --watch
```

### Stream job events (`--stream`)

This prints job related events as they happen and then shows the final job:

```bash
ivcap --output json job create <service-id> -f ./job.json --stream
```

## Option B: Monitor later with `job get`

### 1) Fetch current state

```bash
ivcap --output json job get <job-id-or-urn>
```

### 2) Poll until completion

There is no dedicated `job get --watch` flag currently, so agents should poll safely.

Example (simple polling loop):

```bash
while true; do
  ivcap --output json job get <job-id-or-urn> | jq '{id:.id, status:.status, started_at:.started_at, finished_at:.finished_at}'
  # stop when status is not scheduled/executing
  st=$(ivcap --output json job get <job-id-or-urn> | jq -r '.status // ""')
  if [ "$st" != "scheduled" ] && [ "$st" != "executing" ] && [ "$st" != "" ]; then
    break
  fi
  sleep 2
done
```

Notes:
- Tune `sleep` to reduce API load.
- If the job status vocabulary changes, adjust the terminal condition accordingly.
