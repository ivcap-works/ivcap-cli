## ivcap context dataset grant

Grant read or write access to a dataset

### Synopsis

Grant a project, a service principal or everyone access to a dataset. Granting
to everyone is read-only. Granting to a project needs share rights on the
dataset and write access on that project.

```
ivcap context dataset grant dataset_urn (--project <urn> | --service <urn> | --everyone) --access read|write [flags]
```

### Options

```
      --access string    Access level: read or write
      --everyone         Grant to everyone (read-only)
  -h, --help             help for grant
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

