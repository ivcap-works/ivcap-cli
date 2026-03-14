---
name: ivcap-datafabric-update
version: 0.1.0
description: Update an aspect for an entity+schema pair using file/stdin, with query-first safety checks.
requires:
  bins: ["ivcap"]
---

# Skill: Update an aspect (Data Fabric update) (agent-safe)

`ivcap datafabric update` is a **mutation** that updates an aspect record for a given **entity** and **schema**.

Important: the command only succeeds if there is **exactly one active record** for that entity/schema pair.

## Safety rules
1. Confirm intent with the user.
2. Query first to ensure you have exactly one matching active aspect.
3. Prefer stdin (`-f -`) to avoid temp files.

## Steps

### 1) Query to confirm the target (must be exactly one)

```bash
ivcap --output json datafabric query --entity <entity-urn> --schema-prefix <schema-urn-or-prefix> --limit 10
```

If multiple are returned, do **not** proceed until the user clarifies which record should remain active (you may need `retract` first).

### 2) Update the aspect

From file:

```bash
ivcap --output json datafabric update <entity-urn> -s <schema-urn> -f ./aspect.json --format json
```

Via stdin (agent-friendly):

```bash
cat ./aspect.json | ivcap --output json datafabric update <entity-urn> -s <schema-urn> -f - --format json
```

### 3) Verify

```bash
ivcap --output json datafabric query --entity <entity-urn> --schema-prefix <schema-urn-or-prefix> --include-content --limit 10
```
