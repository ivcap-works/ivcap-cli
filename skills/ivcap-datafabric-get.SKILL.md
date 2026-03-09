---
name: ivcap-datafabric-get
version: 0.1.0
description: Fetch (download) an aspect from the Data Fabric as JSON/YAML, optionally content-only.
requires:
  bins: ["ivcap"]
---

# Skill: Download an aspect (Data Fabric get) (agent-safe)

Aspects are structured records stored in the **Data Fabric**.

## Best practices
- Use `--output json` to keep output machine-readable.
- Prefer explicit aspect URNs (`urn:ivcap:aspect:...`) copied from JSON outputs.

## Examples

### 1) Get the full aspect record

```bash
ivcap --output json datafabric get <aspect-urn>
```

### 2) Get only the content payload

Useful when you want the domain payload without metadata/envelope:

```bash
ivcap --output json datafabric get <aspect-urn> --content-only
```

### 3) Save to a local file (explicit)

```bash
ivcap --output json datafabric get <aspect-urn> > ./aspect.json
```
