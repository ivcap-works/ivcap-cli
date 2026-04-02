## MCP tool: aspect_create

`aspect_create` is a **built-in** tool exposed by `ivcap mcp`.

It creates (adds) a new aspect record for an entity and schema.

### Parameters

- `entity` (required): entity URN/ID
- `schema` (required): schema URN
- `policy` (optional): access policy
- `body` (required): aspect content object
  - If `body.$schema` is not present, the server injects it from the `schema` parameter.

### Example

```json
{
  "entity": "urn:ivcap:entity:...",
  "schema": "urn:ivcap:schema:my.schema.1",
  "body": {
    "my": "content"
  }
}
```
