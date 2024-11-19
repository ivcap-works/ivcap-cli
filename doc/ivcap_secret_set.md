## ivcap secret set

Set a single secret value, overwrite if already exists

```
ivcap secret set [flags] secret-name  secret-value
```

### Options

```
  -e, --expire string   secret expires in the format of '6h', '5d', '100m', '1040s'
  -f, --file string     read secret from file
  -h, --help            help for set
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

* [ivcap secret](ivcap_secret.md)	 - Set and list secrets 

###### Auto generated by spf13/cobra on 29-Oct-2024