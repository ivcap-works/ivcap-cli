## ivcap invitation capabilities

List the capabilities that can be granted via an invitation or grant

### Synopsis

List the grantable capabilities per target kind (project, account), as defined by the platform authorization model.

```
ivcap invitation capabilities [flags]
```

### Options

```
  -h, --help          help for capabilities
      --kind string   Limit to a single kind: project | account
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

* [ivcap invitation](ivcap_invitation.md)	 - Manage project and account invitations

