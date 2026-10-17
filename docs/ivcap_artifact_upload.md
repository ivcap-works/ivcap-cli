## ivcap artifact upload

Resume uploading artifact content

### Synopsis

Resume uploading content to an existing artifact.

Local ZIP, TAR and TAR.GZ archives are indexed before upload and receive an RO-Crate 1.2 JSON-LD
manifest aspect by default. Use --without-manifest to opt out. Content read from stdin cannot be
indexed and is uploaded without a manifest.

```
ivcap artifact upload artifactID -f file|- [flags]
```

### Options

```
      --chunk-size int           Chunk size for splitting large files (default 10000000)
  -t, --content-type string      Content type of artifact; auto-detected when omitted. Accepts a full MIME type (e.g. application/x-compressed-tar) or a bare extension (e.g. tgz, nc, fasta)
  -f, --file string              Path to file containing artifact content
  -h, --help                     help for upload
      --manifest-schema string   Schema URN for the archive manifest aspect (default "urn:ivcap:schema:artifact.archive-manifest.1")
      --without-manifest         Do not add a file manifest for ZIP, TAR or TAR.GZ archives
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

