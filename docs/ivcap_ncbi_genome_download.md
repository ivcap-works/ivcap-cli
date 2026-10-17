## ivcap ncbi genome download

Download the data package of one genome assembly as a zip file

### Synopsis

Download the NCBI Datasets data package of exactly one genome assembly.

ACCESSION must be an assembly accession (GCF_... or GCA_...), with or without a version. A
BioProject accession is rejected, because a project can name many genomes: use
'ivcap ncbi genome reports' to see which assemblies it has, then pass one of them.

The package contains the genomic FASTA by default; add annotation files and reports with
--include. By default it is written to ./<ACCESSION>.zip and the command refuses to replace an
existing file unless --force is given. Use '-f -' to write the zip to stdout.

With --output json|yaml the result (accession, file, size and the list of files in the package)
is printed in machine-readable form.

--timeout does not apply to the transfer itself; stop it with Ctrl-C.

```
ivcap ncbi genome download ACCESSION [flags] [-f file|-]
```

### Examples

```
  ivcap ncbi genome download GCF_000001735.4
  ivcap ncbi genome download GCF_000001735.4 --include genome,gff3,seq-report -f TAIR10.1.zip
  ivcap --output json ncbi genome download GCF_000001735.4 --include all
```

### Options

```
      --assembly-version string   Version policy: 'current', 'latest' or 'all' [use the version given in ACCESSION]
      --chromosomes strings       Limit the genome FASTA to these chromosomes (comma-separated, or 'all')
      --fast-zip-validation       Only check the zip's directory after the download, not every file in it
  -f, --file string               File to write the package to [<ACCESSION>.zip], or - for stdout
      --force                     Overwrite the file if it already exists
  -h, --help                      help for download
      --include strings           Files to include (comma-separated): genome, rna, protein, cds, gff3, gtf, gbff, seq-report, all, none (default [genome])
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

* [ivcap ncbi genome](ivcap_ncbi_genome.md)	 - Genome assemblies at NCBI

