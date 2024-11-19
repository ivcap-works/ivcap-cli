## ivcap secret

Set and list secrets 

### Options

```
  -h, --help   help for secret
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
* [ivcap secret get](ivcap_secret_get.md)	 - Get single secret, show its expiry time and sha1 value
* [ivcap secret list](ivcap_secret_list.md)	 - List existing secrets
* [ivcap secret set](ivcap_secret_set.md)	 - Set a single secret value, overwrite if already exists

###### Auto generated by spf13/cobra on 29-Oct-2024