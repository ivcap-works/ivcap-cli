## ivcap collection add

Add item(s) to an existing collection

### Synopsis

Add one or more items to an existing collection.

Items can be specified in two ways (both may be combined):

  1. As positional URN arguments after the collection URN.
  2. Via --dir: a directory path or glob pattern. Each matching file is
     uploaded as an artifact (skipped if already uploaded) and the resulting
     artifact URN is used as the item.

A 'collection-item' aspect (urn:ivcap:schema:collection-item.1) is created for
each new item. Duplicates (same collection + item) are detected and skipped.

```
ivcap collection add collectionURN [urn...] [--dir <dir-or-glob>] [flags]
```

### Options

```
      --dir string   Directory or glob pattern of files to upload and add
  -h, --help         help for add
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

* [ivcap collection](ivcap_collection.md)	 - Create and manage collections

