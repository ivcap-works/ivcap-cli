## ivcap invitation create

Invite a user to a project or an account

### Synopsis

Invite a user (by email) to a project or an account.

Exactly one target must be given:
  --project <urn>   invite into a project (capabilities: 'ivcap invitation capabilities --kind project')
  --account <urn>   invite into an account (capabilities: 'ivcap invitation capabilities --kind account')

Capabilities may be passed with repeated --capability flags. If none are given and
you are on an interactive terminal, you will be prompted to choose from the valid
set for the target kind. Provided capabilities are validated before the invitation
is created.

```
ivcap invitation create (--project <urn> | --account <urn>) --email <email> [--capability <cap> ...] [flags]
```

### Options

```
      --account string       Account URN to invite the user into
  -c, --capability strings   Capability to grant on accept (repeatable). Run 'ivcap invitation capabilities' to list valid values
  -e, --email string         Invitee email address
  -h, --help                 help for create
      --project string       Project URN to invite the user into
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

