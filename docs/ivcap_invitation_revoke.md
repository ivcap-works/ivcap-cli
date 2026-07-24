## ivcap invitation revoke

Cancel a pending invitation you issued

### Synopsis

Cancel a pending invitation you issued to a project or account. List the
outstanding invitations on a target with 'ivcap project invitations <project>' or
'ivcap account invitations <account>' to find the id.

```
ivcap invitation revoke invitation_id [flags]
```

### Options

```
  -h, --help   help for revoke
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

* [ivcap invitation](ivcap_invitation.md)	 - Respond to invitations addressed to you

