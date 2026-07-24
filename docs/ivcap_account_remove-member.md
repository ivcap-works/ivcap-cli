## ivcap account remove-member

Remove a user from an account entirely (revokes all their grants)

```
ivcap account remove-member account_id --user <urn> [flags]
```

### Options

```
  -h, --help          help for remove-member
      --user string   User URN to remove from the account
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

