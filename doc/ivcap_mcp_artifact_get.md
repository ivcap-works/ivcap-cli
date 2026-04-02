## MCP tool: artifact_get

`artifact_get` is a **built-in** tool exposed by `ivcap mcp`.

It fetches artifact bytes, and can optionally extract a file from inside a tar/tar.gz artifact.

### Parameters

- `id` (required): Artifact URN/ID
- `path` (optional): When provided and the artifact is a tar/tar.gz, returns only the file at that path.

### Caching behavior

If the artifact looks like a tar/tar.gz, the server keeps an in-process cache of the **last** tar artifact downloaded (keyed by artifact id). Subsequent `artifact_get` calls for the same artifact but different `path` are served from this cache.

### Example

Fetch whole artifact:

```json
{ "id": "urn:ivcap:artifact:..." }
```

Fetch a file inside a tar.gz artifact:

```json
{ "id": "urn:ivcap:artifact:...", "path": "data/contract.pdf" }
```
