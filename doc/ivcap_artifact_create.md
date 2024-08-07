## ivcap artifact create

Create a new artifact

```
ivcap artifact create [flags] -n name -f file|-
```

### Options

```
      --chunk-size int        Chunk size for splitting large files (default 10000000)
  -c, --collection string     Assigns artifact to a specific collection
  -t, --content-type string   Content type of artifact
  -f, --file string           Path to file containing artifact content
      --force                 Force creation of new artifact, even if already uploaded
  -h, --help                  help for create
  -n, --name string           Human friendly name
  -p, --policy string         Policy controlling access
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

* [ivcap artifact](ivcap_artifact.md)	 - Create and manage artifacts

###### Auto generated by spf13/cobra on 16-Jul-2024
