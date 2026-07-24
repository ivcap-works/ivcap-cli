## ivcap project create

Create a new project

```
ivcap project create --name <name> [--account-id <urn>] [flags]
```

### Options

```
      --account-id string   Owning account URN
  -h, --help                help for create
  -n, --name string         Display name for the new project
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

* [ivcap project](ivcap_project.md)	 - Manage projects and select the current one

