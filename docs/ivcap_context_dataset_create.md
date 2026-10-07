## ivcap context dataset create

Create a dataset owned by a project

```
ivcap context dataset create --name <name> [--description <text>] [--project <urn>] [flags]
```

### Options

```
  -d, --description string   Description
  -h, --help                 help for create
  -n, --name string          Display name for the new dataset
      --project string       Owning project URN (default: the current project)
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

