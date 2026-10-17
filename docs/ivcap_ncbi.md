## ivcap ncbi

Look up and download genomes from NCBI

### Synopsis

Look up genome assemblies at NCBI and download their data packages.

These commands talk to NCBI Datasets (https://www.ncbi.nlm.nih.gov/datasets/), not to an
IVCAP deployment, so they need no context and no login.

### Options

```
  -h, --help                  help for ncbi
      --ncbi-api-key string   NCBI API key, which raises NCBI's rate limit [NCBI_API_KEY]
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

* [ivcap](ivcap.md)	 - A command line tool to interact with a IVCAP deployment
* [ivcap ncbi genome](ivcap_ncbi_genome.md)	 - Genome assemblies at NCBI

