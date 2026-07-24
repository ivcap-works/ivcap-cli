## ivcap project revoke-capability

Revoke one or more capabilities from a project principal

### Synopsis

Revoke individual project capabilities from a principal, leaving their remaining
capabilities intact. To remove a principal from the project entirely, use
'ivcap project remove-member'.

```
ivcap project revoke-capability project_id (--user <urn> | --service <urn>) --capability <cap> ... [flags]
```

### Options

```
  -c, --capability strings   Capability to revoke (repeatable)
  -h, --help                 help for revoke-capability
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

