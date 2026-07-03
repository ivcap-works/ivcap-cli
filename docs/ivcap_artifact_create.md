## ivcap artifact create

Create a new artifact

### Synopsis

Create a new artifact by uploading file content to IVCAP.

The MIME content-type is auto-detected from the file header when -t/--content-type
is omitted.  Detection uses the 'mimetype' library (500+ formats) with extension-based
overrides for ambiguous types (e.g. .tar.gz → application/x-compressed-tar).

When -t is provided it accepts either a full MIME type or a bare file extension:

  -t application/x-compressed-tar   full MIME type
  -t tgz                            bare extension (same result)
  -t .tar.gz                        extension with dot (same result)

Supported extension shorthands include (but are not limited to):
  Archives:     tar, tgz/tar.gz, gz, bz2, xz, zst
  Earth/env:    nc/nc4, fits/fit/fts, hdf5/h5, zarr
  Bioinformat.: fasta/fa/fna/faa, fastq/fq, bam, sam, vcf, bcf, bed, gff, gff3, gtf
  Data science: parquet, arrow, mat, npy, npz

When reading from stdin (-f -) the first 512 bytes are sniffed and replayed into the
upload stream so no data is lost.  For piped tar.gz streams (where the extension is
unavailable) pass -t tgz or -t application/x-compressed-tar explicitly.

```
ivcap artifact create [flags] -n name -f file|-
```

### Options

```
      --chunk-size int        Chunk size for splitting large files (default 10000000)
  -c, --collection string     Assigns artifact to a specific collection
  -t, --content-type string   Content type of artifact; auto-detected when omitted. Accepts a full MIME type (e.g. application/x-compressed-tar) or a bare extension (e.g. tgz, nc, fasta)
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

