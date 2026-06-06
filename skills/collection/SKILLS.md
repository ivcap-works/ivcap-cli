# Collection skills

Skills related to creating and managing IVCAP collections via the DataFabric aspect API.

> ⚠️ **MCP note:** IVCAP does **not** expose dedicated `collection` MCP tools.
> Agents must use the DataFabric aspect tools (`aspect_create`, `aspect_search`,
> `aspect_get`) directly, together with the collection schemas documented here.

## Schemas

| Schema URN | Purpose |
|---|---|
| `urn:ivcap:schema:collection.1` | Collection definition (name + description) |
| `urn:ivcap:schema:collection-item.1` | Membership record (one per item in the collection) |

Both schema aspects are attached to the **collection entity URN** — the URN you
choose for the collection is used as the `entity` field for all aspect calls.

## Included skills

- Create a collection: `skills://ivcap-collection-create/SKILL.md`
- Add items to a collection: `skills://ivcap-collection-add/SKILL.md`
- List and inspect collections: `skills://ivcap-collection-query/SKILL.md`

## Removing items / retracting collections (CLI only)

There is currently **no MCP tool** for retracting aspects. Use the CLI:

```bash
# Remove a single item membership aspect
ivcap --output json datafabric retract <aspect-urn>

# CLI collection commands (full workflow helpers)
ivcap collection remove <collectionURN> <itemURN> [<itemURN>...]
ivcap collection retract <collectionURN>   # retracts all items + the definition
```
