## ivcap mcp

Start an MCP server for accessing all tools on an IVCAP platform

### Synopsis

Start an MCP (Model Context Protocol) server.

In addition to MCP Tools for calling platform services, this server also exposes
the ivcap-cli release's embedded agent skills via MCP Resources and Prompts
("MCP-Provisioned Skills").

Resources (preferred if your MCP client exposes resources/read to LLMs):
  - skills://manifest         JSON manifest of available skills
  - skills://catalog.json     JSON catalog (metadata + hashes; no bodies)
  - skills://CONTEXT.md       General agent best-practices for ivcap-cli
  - skills://SKILLS.md        Top-level skills tree index
  - skills://{name}/SKILL.md  Skill playbook body (markdown)
  - skills://file/{path}      Any embedded markdown file (e.g. category SKILLS.md)

Bridge Tools (fallback for clients that don't expose resources/read):
  - list_skills               List available skill playbooks (same as skills://manifest)
  - read_skill                Read a skill playbook by name (same as skills://{name}/SKILL.md)

Prompts:
  - use-ivcap-best-practices  Instructs an agent to load CONTEXT + relevant skills
                              (prefers resources/read, falls back to tools if needed)

Note: Many MCP clients (including Claude Desktop) don't currently expose resources/read
to LLMs for programmatic access. The bridge tools provide equivalent functionality via
tools/call until client support improves.

Recommended MCP client system prompt:

Before answering any task:
1. Call resources/list on all connected MCP servers (if available)
2. Identify resources matching: *SKILL.md, *instructions*, *prompt*
3. Fetch and read matching resources via resources/read (if available)
4. Otherwise use list_skills and read_skill tools
5. Apply those instructions when completing the user's request


```
ivcap mcp [flags]
```

### Options

```
  -h, --help                 help for mcp
      --log-dir string       directory for MCP log files (default "/tmp")
      --port int             optional port to open for SSE connection to MCP server (default -1)
  -s, --tool-schema string   the schema URN used for describing MCP tools (default "urn:sd-core:schema.ai-tool.1")
      --with-logging         enable JSON-RPC request/response logging to file
```

### Options inherited from parent commands

```
      --access-token string   Access token to use for authentication with API server [IVCAP_ACCESS_TOKEN]
      --context string        Context (deployment) to use
      --debug                 Set logging level to DEBUG
      --no-history            Do not store history
  -o, --output string         Set format for displaying output [json, yaml]
      --silent                Do not show any progress information
      --timeout int           Max. number of seconds to wait for completion (default 30)
```

### SEE ALSO

* [ivcap](ivcap.md)	 - A command line tool to interact with a IVCAP deployment

