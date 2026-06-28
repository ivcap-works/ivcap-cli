## ivcap nextflow retract

Retract the service aspect(s) created by 'nextflow create'

### Synopsis

Query and retract the service description aspect(s) for a given service ID. This is the opposite of 'nextflow create'.

```
ivcap nextflow retract service-id [flags]
```

### Options

```
  -h, --help   help for retract
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

* [ivcap nextflow](ivcap_nextflow.md)	 - Commands for working with Nextflow-based services

