## ivcap context dataset revoke

Revoke a dataset grant

### Synopsis

Revoke one access level from a grantee. Either side may revoke: share rights on
the dataset, or write access on the granted project.

```
ivcap context dataset revoke dataset_urn (--project <urn> | --service <urn> | --everyone) --access read|write [flags]
```

### Options

```
      --access string    Access level: read or write
      --everyone         Grant to everyone (read-only)
  -h, --help             help for revoke
      --project string   Project URN to grant to
      --service string   Service principal URN to grant to
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

* [ivcap context dataset](ivcap_context_dataset.md)	 - Manage datasets and who can access them

