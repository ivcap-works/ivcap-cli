## ivcap capabilities

List the capabilities grantable on projects and accounts

### Synopsis

List the grantable capabilities per target kind (project, account), as defined by
the platform authorization model. These are the values accepted by the --capability
flag of 'ivcap project grant', 'ivcap account grant', and the 'invite' commands.

```
ivcap capabilities [flags]
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

* [ivcap](ivcap.md)	 - A command line tool to interact with a IVCAP deployment

