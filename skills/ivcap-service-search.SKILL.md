---
name: ivcap-service-search
version: 0.1.0
description: Convenience alias for searching services. Agents should prefer `service list --search`.
requires:
  bins: ["ivcap"]
---

# Skill: Service search (alias)

`ivcap service search` is a **human convenience alias** for:

```bash
ivcap service list --search <query>
```

It joins all remaining CLI arguments with spaces to form `<query>`.

## Agent guidance (important)

- **Do not use `ivcap service search` in agent workflows.**
- Prefer the more generic form below, because it is explicit, uniform across list endpoints, and easier to parameterize:

```bash
ivcap --output json service list --limit 10 --search "<query>"
```

## Examples (preferred)

Search services by a free-text query:

```bash
ivcap --output json service list --limit 10 --search "data fabric"
```

Combine `--search` with structured filtering:

```bash
ivcap --output json service list --limit 10 --search "gene" --filter 'status==Ready'
```
