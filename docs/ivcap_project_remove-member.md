## ivcap project remove-member

Remove a principal from a project entirely (revokes all their capabilities)

```
ivcap project remove-member project_id (--user <urn> | --service <urn>) [flags]
```

### Options

```
  -h, --help             help for remove-member
      --service string   Service principal URN to target
      --user string      User URN to target
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

