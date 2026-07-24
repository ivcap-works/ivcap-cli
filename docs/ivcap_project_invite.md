## ivcap project invite

Invite a user to a project

### Synopsis

Invite a user (by email) to a project, granting the given capabilities when they
accept. If no capabilities are given and you are on an interactive terminal, you
will be prompted to choose from the valid set.

```
ivcap project invite project_id --email <email> [--capability <cap> ...] [flags]
```

### Options

```
  -c, --capability strings   Capability to grant on accept (repeatable). Run 'ivcap capabilities --kind project' to list valid values
  -e, --email string         Invitee email address
  -h, --help                 help for invite
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

