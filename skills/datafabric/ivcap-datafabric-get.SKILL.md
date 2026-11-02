---
name: ivcap-datafabric-get
version: 0.2.0
description: Fetch the content of a datafabric item (aspect), or with --full-record the whole record.
requires:
  bins: ["ivcap"]
---

# Skill: datafabric get (agent-safe)

Prints the content of one aspect. By default only the content is printed: as YAML, or as JSON with `--output json`.

```bash
ivcap --output json datafabric get <aspect-urn>
```

Use `--full-record` when you need more than the content: the entire record with its `id`, `entity`, `schema`,
`content-type`, `asserter`, validity times (`valid-from`, `valid-to`) and the `content`.

```bash
ivcap --output json datafabric get --full-record <aspect-urn>
```

The URN can also be an `@n` reference from an earlier listing. The same applies to
`ivcap datafabric query --get-if-one`: it prints only the content unless `--full-record` is given.
