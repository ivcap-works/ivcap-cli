## MCP tool: aspect_search

`aspect_search` is a **built-in** tool exposed by `ivcap mcp`.

It lists/searches aspects in the DataFabric.

### Parameters

- `entity` (optional): filter by entity URN/ID
- `schema_prefix` (optional): filter by schema URN/prefix
- `include_content` (optional, default `false`): include aspect `content` in the response
- `content_path` (optional): JSON-path filter applied to aspect content (passed through as `aspect-path`)
- `limit` (optional): page size
- `page` (optional): paging cursor

### Example

List aspects for an entity (metadata only):

```json
{ "entity": "urn:ivcap:entity:...", "limit": 20 }
```

List and include content:

```json
{ "entity": "urn:ivcap:entity:...", "include_content": true }
```
