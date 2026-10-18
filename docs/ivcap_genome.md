## ivcap genome

Look up, download and import genomes from NCBI

### Synopsis

Look up genome assemblies at NCBI, download their data packages and import them into IVCAP.

The download and reports commands only talk to NCBI Datasets
(https://www.ncbi.nlm.nih.gov/datasets/), so they need no IVCAP context or login. Import uploads
an existing package to the active IVCAP deployment and attaches report and manifest aspects.
List shows what has been imported and needs the same context and login as import.

### Options

```
  -h, --help   help for genome
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
* [ivcap genome download](ivcap_genome_download.md)	 - Download the data package of one genome assembly as a zip file
* [ivcap genome import](ivcap_genome_import.md)	 - Import an NCBI genome package into IVCAP
* [ivcap genome list](ivcap_genome_list.md)	 - List the genomes imported into IVCAP
* [ivcap genome reports](ivcap_genome_reports.md)	 - Show the NCBI genome assembly reports for assembly or BioProject accessions

