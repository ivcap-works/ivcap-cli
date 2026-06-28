## ivcap artifact tar download

Download a specific file from a tar/tar.gz artifact

```
ivcap artifact tar download artifact_id file_path [-f output_file] [--cache] [flags]
```

### Options

```
      --cache         Cache the artifact locally for future access
  -f, --file string   File to write content to [stdout]
  -h, --help          help for download
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

* [ivcap artifact tar](ivcap_artifact_tar.md)	 - Manage tar/tar.gz artifacts

