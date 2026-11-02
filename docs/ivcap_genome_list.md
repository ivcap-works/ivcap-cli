## ivcap genome list

List the genomes imported into IVCAP

### Synopsis

List the genomes that 'ivcap genome import' stored in IVCAP, one row per import.

A genome is listed when the data fabric holds an assembly report aspect for it. Each row shows the
accession from that report, the report aspect, and the artifact the aspect belongs to, which holds the
imported package. Importing an accession twice gives two rows. Use --report-schema if the imports used
another report schema.

The report aspect and the artifact are shown as short references such as @3, which other commands accept in
place of the URN. Use --no-history to show the full URNs instead.

The paging, ordering and time flags are the same as for the other list commands. A last row such as
"... @5" is the reference to the next page: pass it to --page.

With --output json|yaml the same rows, with the organism and assembly name of each report, are printed as 'genomes', plus 'next_page' when there are more
results; pass that to --page to continue.

```
ivcap genome list [flags]
```

### Examples

```
  ivcap genome list
  ivcap --output json genome list --limit 50
```

### Options

```
      --at-time string         query state at this time in the past
      --filter string          filter list (e.g. "name~=Fred")
  -h, --help                   help for list
  -l, --limit int              max number of records to be returned (default 10)
      --order-by string        feature to order list by (e.g. "created-at,status")
      --order-desc             if set, order in descending order
  -p, --page string            page cursor
      --report-schema string   Schema URN of the assembly report aspects to list (default "urn:ivcap:schema:ncbi.assembly-data-report.1")
      --search string          search list
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

* [ivcap genome](ivcap_genome.md)	 - Look up, download and import genomes from NCBI

