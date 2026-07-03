## ivcap artifact create

Create a new artifact

### Synopsis

Create a new artifact by uploading file content to IVCAP.

The MIME content-type is auto-detected from the file header when -t/--content-type
is omitted.  Detection uses the 'mimetype' library (500+ formats) with the following
extension-based overrides for types whose magic bytes are ambiguous:

  .tar.gz / .tgz  →  application/x-compressed-tar
  .nc / .nc4      →  application/netcdf

When reading from stdin (-f -) the first 512 bytes are sniffed and then replayed
into the upload stream so no data is lost.  For piped tar.gz streams where the
extension is unavailable, pass -t application/x-compressed-tar explicitly.

```
ivcap artifact create [flags] -n name -f file|-
```

### Options

```
      --chunk-size int        Chunk size for splitting large files (default 10000000)
  -c, --collection string     Assigns artifact to a specific collection
  -t, --content-type string   Content type of artifact (auto-detected from file header when omitted)
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

