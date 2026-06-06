## ivcap collection retract

Fully retract a collection and all its item memberships

### Synopsis

Fully retract a collection from the DataFabric.

All collection-item aspects (urn:ivcap:schema:collection-item.1) for the
collection are retracted first, then the collection definition aspect
(urn:ivcap:schema:collection.1) itself is retracted.

This operation cannot be undone.

```
ivcap collection retract collectionURN [flags]
```

### Examples

```
# Retract a collection and all its items
ivcap collection retract urn:ivcap:collection:...

# Using a history shortcut
ivcap collection retract @-1

# Suppress progress output
ivcap collection retract urn:ivcap:collection:... --silent
```

### Options

```
  -h, --help   help for retract
```

### Options inherited from parent commands

```
      --access-token string   Access token to use for authentication
      --context string        Context (deployment) to use
  -o, --output string         Set output format [json, yaml, table]
  -q, --silent                Do not print info/progress messages
      --timeout duration      Timeout for this request (default 30s)
```

### SEE ALSO

* [ivcap collection](ivcap_collection.md) - Create and manage collections
* [ivcap collection remove](ivcap_collection_remove.md) - Remove item(s) from a collection
* [ivcap collection add](ivcap_collection_add.md) - Add item(s) to an existing collection
