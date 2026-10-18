## ivcap datafabric get

Get the content of a specific aspect

### Synopsis

Print the content of the aspect with the given URN (or @n reference).

By default only the content is printed: as YAML, or as JSON with --output json. Use --full-record to
print the entire record instead: its id, entity, schema, content type, asserter, validity times and the
content. Without --output the full record is shown as a table.

```
ivcap datafabric get aspectURN [flags]
```

### Examples

```
  ivcap datafabric get @1
  ivcap --output json datafabric get @1
  ivcap datafabric get --full-record @1
  ivcap --output json datafabric get --full-record @1
```

### Options

```
      --full-record   print the entire aspect record (id, entity, schema, validity, ... and content), not only its content
  -h, --help          help for get
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

* [ivcap datafabric](ivcap_datafabric.md)	 - Query the datafabric and create and manage aspects within

