## ivcap invitation

Manage project and account invitations

### Synopsis

Manage invitations to projects and accounts.

An invitation grants an email address a set of capabilities on a target when they
accept it. The target is either a PROJECT (`--project`) or an ACCOUNT
(`--account`); the valid capabilities differ by target kind. List them with:

    ivcap invitation capabilities

Invitees manage invitations addressed to them with 'list', 'accept' and 'decline'.

### Options

```
  -h, --help   help for invitation
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

* [ivcap](ivcap.md)	 - A command line tool to interact with a IVCAP deployment
* [ivcap invitation accept](ivcap_invitation_accept.md)	 - Accept an invitation addressed to you
* [ivcap invitation capabilities](ivcap_invitation_capabilities.md)	 - List the capabilities that can be granted via an invitation or grant
* [ivcap invitation create](ivcap_invitation_create.md)	 - Invite a user to a project or an account
* [ivcap invitation decline](ivcap_invitation_decline.md)	 - Decline an invitation addressed to you
* [ivcap invitation list](ivcap_invitation_list.md)	 - List invitations addressed to you

