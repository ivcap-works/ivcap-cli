## ivcap account grant

Grant account capabilities to a user

### Synopsis

Grant one or more account-admin capabilities to an existing member. List the
grantable capabilities with 'ivcap capabilities --kind account'.

```
ivcap account grant account_id --user <urn> --capability <cap> ... [flags]
```

### Options

```
  -c, --capability strings   Capability to grant (repeatable). Run 'ivcap capabilities --kind account' to list valid values
  -h, --help                 help for grant
      --user string          User URN to grant capabilities to
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

* [ivcap account](ivcap_account.md)	 - Manage accounts you belong to

