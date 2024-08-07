## ivcap service update

Update an existing service

### Synopsis

Update an existing service description or create it if it does not exist
AND the --create flag is set. If the service definition is provided
through 'stdin' use '-' as the file name and also include the --format flag

```
ivcap service update [flags] service-id -f service-file|-
```

### Options

```
      --create          Create service record if it doesn't exist
  -f, --file string     Path to service description file
      --format string   Format of input file [json, yaml] (default "json")
  -h, --help            help for update
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

* [ivcap service](ivcap_service.md)	 - Create and manage services

###### Auto generated by spf13/cobra on 16-Jul-2024
