## ivcap project grant

Grant project capabilities to a user or service principal

### Synopsis

Grant one or more project capabilities to an existing member (user or service
principal). List the grantable capabilities with 'ivcap capabilities --kind project'.

```
ivcap project grant project_id (--user <urn> | --service <urn>) --capability <cap> ... [flags]
```

### Options

```
  -c, --capability strings   Capability to grant (repeatable). Run 'ivcap capabilities --kind project' to list valid values
  -h, --help                 help for grant
      --service string       Service principal URN to target
      --user string          User URN to target
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

