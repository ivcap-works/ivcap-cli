## ivcap ncbi genome reports

Show the NCBI genome assembly reports for assembly or BioProject accessions

### Synopsis

Look up the NCBI genome assembly report of each assembly named by the accessions.

Accessions may be assembly accessions (GCF_... or GCA_...) or BioProject accessions (PRJNA...);
a BioProject expands to its assemblies. Use this to find the assembly you want to download.

With --output json|yaml the full reports are printed. Each report follows the JSON Schema at
https://github.com/ivcap-works/ivcap-ncbi-genome-download/tree/v0.2.0/schema

```
ivcap ncbi genome reports ACCESSION... [flags]
```

### Examples

```
  ivcap ncbi genome reports GCF_000001735.4
  ivcap --output json ncbi genome reports GCF_000001735.4
  ivcap ncbi genome reports PRJNA10719 --reference
```

### Options

```
      --annotated                 Limit to annotated genomes
      --assembly-level strings    Limit to assembly levels (comma-separated): chromosome, complete, contig, scaffold
      --assembly-source string    Limit to 'refseq' (GCF_) or 'genbank' (GCA_) assemblies (default "all")
      --assembly-version string   Version policy: 'current', 'latest' or 'all' [use the versions given]
      --exclude-atypical          Exclude atypical assemblies
      --exclude-multi-isolate     Exclude assemblies from multi-isolate projects
      --from-type                 Only return records with type material
  -h, --help                      help for reports
      --mag string                Limit to metagenome assembled genomes ('only') or remove them ('exclude') (default "all")
      --reference                 Limit to reference genomes
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

