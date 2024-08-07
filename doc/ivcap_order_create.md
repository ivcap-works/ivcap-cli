## ivcap order create

Create a new order

### Synopsis

Create a new order for a service identified by it's id and add any
potential paramters using the format 'paramName=value'. Please not that there
cannot be any spaces between the parameter name, the '=' and the value. If the value
contains spaces, put it into quotes which will not be removed by your shell.

An example:

  ivcap order create --name "test order" ivcap:service:d939b74d-0070-59a4-a832-36c5c07e657d msg="Hello World"



```
ivcap order create [flags] service-id [... paramName=value]
```

### Options

```
      --account-id string      override the account ID to use for this request
  -h, --help                   help for create
  -n, --name string            Human friendly name
      --skip-parameter-check   fskip checking order paramters first ONLY USE FOR TESTING
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

* [ivcap order](ivcap_order.md)	 - Create and manage orders

###### Auto generated by spf13/cobra on 16-Jul-2024
