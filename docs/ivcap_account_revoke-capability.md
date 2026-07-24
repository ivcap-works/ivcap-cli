## ivcap account revoke-capability

Revoke one or more capabilities from an account member

### Synopsis

Revoke individual account capabilities from a user, leaving their remaining
capabilities intact. To remove a member from the account entirely, use
'ivcap account remove-member'.

```
ivcap account revoke-capability account_id --user <urn> --capability <cap> ... [flags]
```

### Options

```
  -c, --capability strings   Capability to revoke (repeatable)
  -h, --help                 help for revoke-capability
      --user string          User URN to revoke from
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

