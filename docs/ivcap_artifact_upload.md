## ivcap artifact upload

Resume uploading artifact content

```
ivcap artifact upload artifactID -f file|- [flags]
```

### Options

```
      --chunk-size int        Chunk size for splitting large files (default 10000000)
  -t, --content-type string   Content type of artifact; auto-detected when omitted. Accepts a full MIME type (e.g. application/x-compressed-tar) or a bare extension (e.g. tgz, nc, fasta)
  -f, --file string           Path to file containing artifact content
  -h, --help                  help for upload
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

