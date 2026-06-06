## ivcap collection remove

Remove item(s) from a collection

### Synopsis

Remove one or more items from an existing collection.

For each item URN provided, the corresponding collection-item aspect
(urn:ivcap:schema:collection-item.1) is retracted from the DataFabric.

Items that are not currently members of the collection are silently skipped.

```
ivcap collection remove collectionURN urn [urn...] [flags]
```

### Examples

```
# Remove a single artifact from a collection
ivcap collection remove urn:ivcap:collection:... urn:ivcap:artifact:...

# Remove multiple items at once
ivcap collection remove urn:ivcap:collection:... urn:ivcap:artifact:aaa urn:ivcap:artifact:bbb

# Use history shortcuts
ivcap collection remove @-1 @-2
```

### Options

```
  -h, --help   help for remove
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
* [ivcap collection add](ivcap_collection_add.md) - Add item(s) to an existing collection
