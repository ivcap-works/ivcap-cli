## ivcap datafabric add

Add aspect of a specific schema to an entity

### Synopsis

.....

```
ivcap datafabric add entityURN [-s schemaName] -f -|aspect --format json|yaml [flags]
```

### Options

```
  -f, --file string     Path to file containing aspect content
      --format string   Format of input file [json, yaml] (default "json")
  -h, --help            help for add
  -p, --policy string   Policy controlling access
  -s, --schema string   URN/UUID of schema
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

###### Auto generated by spf13/cobra on 26-May-2025
