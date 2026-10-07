## ivcap context dataset

Manage datasets and who can access them

### Synopsis

Manage datasets. A dataset belongs to one project, which manages it; other
projects, service principals or everyone can be granted read or write access.
Every project has a default dataset that shares the project's id.

Datasets are addressed by URN (urn:ivcap:dataset:<uuid>).

### Options

```
  -h, --help   help for dataset
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

* [ivcap context](ivcap_context.md)	 - Manage deployment access, projects, and accounts
* [ivcap context dataset create](ivcap_context_dataset_create.md)	 - Create a dataset owned by a project
* [ivcap context dataset delete](ivcap_context_dataset_delete.md)	 - Delete a dataset (not yet supported)
* [ivcap context dataset events](ivcap_context_dataset_events.md)	 - Show a dataset's audit history
* [ivcap context dataset get](ivcap_context_dataset_get.md)	 - Fetch details about a single dataset
* [ivcap context dataset grant](ivcap_context_dataset_grant.md)	 - Grant read or write access to a dataset
* [ivcap context dataset grants](ivcap_context_dataset_grants.md)	 - List who can read or write a dataset
* [ivcap context dataset list](ivcap_context_dataset_list.md)	 - List the datasets a project owns or has been granted access to
* [ivcap context dataset revoke](ivcap_context_dataset_revoke.md)	 - Revoke a dataset grant
* [ivcap context dataset update](ivcap_context_dataset_update.md)	 - Change a dataset's name or description

