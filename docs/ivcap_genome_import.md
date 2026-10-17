## ivcap genome import

Import an NCBI genome package into IVCAP

### Synopsis

Import an NCBI genome data package as an IVCAP artifact.

The package is downloaded to a temporary file which is removed after upload; no local package is
left behind. The same --include, --chromosomes, --assembly-version and --fast-zip-validation options
as 'genome download' control the package. The imported artifact receives an assembly report
aspect and an RO-Crate 1.2 archive manifest aspect.

The manifest records a license for the data. NCBI states none: it places no restrictions itself,
but submitters may claim rights that it cannot assess. By default the manifest therefore links to
NCBI's data usage policy, which is a pointer and not a license grant. Use --license to state another.

If an aspect request fails after upload, the error names the uploaded artifact so it can be repaired.

```
ivcap genome import ACCESSION [flags]
```

### Examples

```
  ivcap genome import GCF_000001735.4
  ivcap genome import GCF_000001735.4 --include genome,gff3,seq-report
  ivcap --output json genome import GCF_000001735.4 --policy urn:ivcap:policy:...
```

### Options

```
      --assembly-version string   Version policy: 'current', 'latest' or 'all' [use the version given in ACCESSION]
      --chromosomes strings       Limit the genome FASTA to these chromosomes (comma-separated, or 'all')
      --chunk-size int            Chunk size in bytes for splitting large files: at least 65536, or -1 to upload without chunking (default 10000000)
      --fast-zip-validation       Only check the zip's directory after the download, not every file in it
  -h, --help                      help for import
      --include strings           Files to include (comma-separated): genome, rna, protein, cds, gff3, gtf, gbff, seq-report, all, none (default [genome])
      --license string            License of the imported data, recorded in the manifest: an SPDX identifier such as CC0-1.0, a license URL, or 'undefined' [a link to NCBI's data usage policy, which states no license]
      --manifest-schema string    Schema URN for the RO-Crate manifest aspect (default "urn:ivcap:schema:artifact.archive-manifest.1")
  -n, --name string               Artifact name [zip file name]
  -p, --policy string             Policy controlling access to the artifact and aspects
      --report-schema string      Schema URN for the assembly report aspect (default "urn:ivcap:schema:ncbi.assembly-data-report.1")
```

### Options inherited from parent commands

```
      --access-token string   Access token to use for authentication with API server [IVCAP_ACCESS_TOKEN]
      --context string        Context (deployment) to use
      --debug                 Set logging level to DEBUG
      --ncbi-api-key string   NCBI API key, which raises NCBI's rate limit [NCBI_API_KEY]
      --no-history            Do not store history
  -o, --output string         Set format for displaying output [json, yaml]
      --silent                Do not show any progress information
      --timeout int           Max. number of seconds to wait for completion (default 30)
```

### SEE ALSO

* [ivcap genome](ivcap_genome.md)	 - Look up, download and import genomes from NCBI

