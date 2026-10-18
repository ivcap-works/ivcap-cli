---
name: ivcap-genome-list
version: 0.1.0
description: List the available genomes (imported into IVCAP) with organism, assembly, assembly report aspect and artifact.
requires:
  bins: ["ivcap"]
---

# Skill: List imported genomes (agent-safe, read-only)

Use this to answer "which genomes are available on IVCAP". It only reads from the active IVCAP deployment and
changes nothing. It needs a logged-in IVCAP context, but no NCBI access. A service search finds deployed analysis
services, not the genomes themselves.

## Best practices
- Use `--output json` when you will parse the result.
- A genome is listed when it has an assembly report aspect. The artifact is the entity of that aspect and holds
  the imported package.
- An accession imported twice appears twice, once per import.
- If the result has `next_page`, pass it to `--page` to get the next batch.

## Example

```bash
ivcap --output json genome list --limit 50
```

The result is `{"genomes": [{"accession": "...", "organism": "...", "assembly_name": "...", "report_aspect":
"urn:ivcap:aspect:...", "artifact": "urn:ivcap:artifact:..."}], "next_page": "..."}`. `organism` and `assembly_name`
come from the assembly report and are left out when it has none; `next_page` is only present when there are more
results. Use `artifact` with `ivcap artifact get` or `ivcap artifact download`.

## Via MCP
The MCP server (`ivcap mcp`) offers the same as the tool `genome_list`, with the optional arguments `limit` and
`page`. It returns `genomes` and, when there are more results, `next_page`.
