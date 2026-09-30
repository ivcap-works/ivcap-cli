## ivcap context create

Create a new context

### Synopsis

Create a named context pointing at an IVCAP deployment.

Pass a bare domain to let the CLI derive both URLs by convention:
  ivcap context create prod develop.ivcap.net
  → API: https://api.develop.ivcap.net
  → Identity: https://id.develop.ivcap.net

Pass a full URL for non-standard deployments (localhost, minikube, SSH tunnels).
The identity URL is derived by stripping any api. prefix and prepending id.;
use --identity-url to override when the convention does not apply:
  ivcap context create local http://localhost:8080 --identity-url http://localhost:8002

```
ivcap context create ctxtName <domain-or-url> [flags]
```

### Options

```
  -h, --help                  help for create
      --host-name string      optional host name if accessing API through SSH tunnel
      --identity-url string   identity server URL (required when the second arg is a full URL, e.g. http://localhost:8002)
      --version int           define API version (default 1)
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

* [ivcap context](ivcap_context.md)	 - Manage deployment access, projects, and accounts

