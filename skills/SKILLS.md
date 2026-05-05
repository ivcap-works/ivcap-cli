# IVCAP Embedded Skills

This ivcap-cli release embeds a small set of **agent skill documents** and **skill indexes**.
They are version-matched to this binary and available offline.

You can read these docs via:

- **CLI**: `ivcap skills show <name-or-uri>`
- **MCP**: `resources/read` on the `skills://...` URIs, or use the **`search`** tool

## ⚠️ CRITICAL: Before Reading a Skill

**ALWAYS verify the skill exists first** by listing available skills:

```bash
# CLI: List all available skills
ivcap skills list

# MCP: Use the search tool with skills='*' to list all skills
# (No authentication required)
```

**Why:** Attempting to read a skill that doesn't exist will fail. Always check the output first to see the exact skill names.

**If you get an error:** ⚠️ **READ THE ERROR MESSAGE VERY CAREFULLY.** Do not ignore it or hallucinate a solution. The error message tells you exactly what went wrong:
- `"skill not found"` → Use `ivcap skills list` to find the correct name
- `"invalid URI format"` → Check the URI syntax in your list output
- `"no such file"` → The referenced skill doesn't exist in this binary

**Example workflow:**
```bash
# 1. List skills to find the exact names
ivcap skills list
# Output shows: nextflow, nextflow-mcp-tools, job-basics, etc.

# 2. Read a skill you verified exists
ivcap skills show nextflow-mcp-tools

# 3. If you get an error, READ IT carefully before trying something else
```

## Start here

- General agent guidance: `skills://CONTEXT.md`

## Reference documentation

- Authentication lifecycle: `skills://file/references/auth.md`

## Skills tree

### Core CLI domains

- Services: `skills://file/service/SKILLS.md`
- Jobs: `skills://file/job/SKILLS.md`
- Artifacts: `skills://file/artifact/SKILLS.md`
- Data Fabric: `skills://file/datafabric/SKILLS.md`

### Pipelines / workflows

- Nextflow: `skills://file/nextflow/SKILLS.md`
