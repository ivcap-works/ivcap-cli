---
name: ivcap-datafabric-retract
version: 0.1.0
description: Retract an aspect record (mutation) with agent-safe confirmation guidance.
requires:
  bins: ["ivcap"]
---

# Skill: Retract an aspect (Data Fabric retract) (agent-safe)

Retracting an aspect is a **mutation**: it removes (retracts) a specific aspect record.

## Safety rules (important)
1. Confirm intent with the user.
2. Use an explicit aspect URN (avoid `@...` history tokens).
3. Prefer doing a `datafabric get` first to confirm you are retracting the correct record.

## Steps

### 1) Inspect the aspect you plan to retract

```bash
ivcap --output json datafabric get <aspect-urn>
```

### 2) Retract the aspect

```bash
ivcap --output json datafabric retract <aspect-urn>
```

### 3) Verify (optional)

Re-query or re-get to confirm it is no longer active/visible:

```bash
ivcap --output json datafabric query --entity <entity-urn> --schema-prefix <schema-prefix> --limit 10
```
