# IVCAP - A Command-line Tool to interact with an IVCAP deployment

```
% ivcap
A command line tool to to more conveniently interact with the
API exposed by a specific IVCAP deployment.

Usage:
  ivcap [command]

Available Commands:
  artifact    Create and manage artifacts 
  completion  Generate the autocompletion script for the specified shell
  config      A brief description of your command
  help        Help about any command
  login       Authenticate with a specific deployment/context
  order       Create and manage orders 
  service     Create and manage services 

Flags:
      --account-id string   Account ID to use with requests. Most likely defined in context
      --context string      Context (deployment) to use
      --debug               Set logging level to DEBUG
  -h, --help                help for ivcap
  -v, --version             version for ivcap

Use "ivcap [command] --help" for more information about a command.
```

## Configure context for a specific deployment

With the following command we are creating a context named `cip-2` for the IVCAP deployment at `https://api2.green-cirrus.com`

```
% ivcap config create-context cip-2 --url https://api2.green-cirrus.com
Context 'cip-2' created.
```

If we have multiple contexts, we can easily switch with `config use-context`

```
% ivcap config use-context cip-2
Switched to context 'cip-2'.
```

The following command lists all the configured contexts:

```
% ivcap config get-contexts                                           
+---------+-----------+-----------------------------+------------------------------+
| CURRENT | NAME      | ACCOUNTID                   | URL                          |
+---------+-----------+-----------------------------+------------------------------+
| *       | cip-2     |                             | http://api2.green-cirrus.com |
+---------+-----------+-----------------------------+------------------------------+
```

To obtain an authorisation token, some deployments provide a username/password based identity provider. The
respective password is requested separately if not provided through the `--login-password` or the `IVCAP_PASSWORD` environment variable.

```
% ivcap login max@testing.com
password: ...
Login succeeded
```

## List Services

```
% ivcap services list --limit 2
+---------------------------------------------------+---------------------+-----------------------------------------------------------------+
| ID                                                | NAME                | PROVIDER                                                        |
+---------------------------------------------------+---------------------+-----------------------------------------------------------------+
| cayp:service:d939b74d-0070-59a4-a832-36c5c07e657d | Gradient Text Image | cayp:provider:1a18fe6b-ffd4-594b-89fb-4c3e8b3ac188:testing.com  |
| cayp:service:74856672-26f1-5df8-9de0-787f6a1fed25 | Windy days.         | cayp:provider:48609f7d-5a64-5bf6-9c47-1a3ad40bf28a:cre.csiro.au |
+---------------------------------------------------+---------------------+-----------------------------------------------------------------+
```

## Place an Order

```
% ivcap orders create \
     cayp:service:d939b74d-0070-59a4-a832-36c5c07e657d \
     --name "Order for max" \
     msg="Hi, how are you"
Order 'cayp:order:81b204e8-c404-499e-bc19-d78518a5a3dc' with status 'Pending' submitted.
```

To check on the status of an order:

```
% ivcap orders get cayp:order:81b204e8-c404-499e-bc19-d78518a5a3dc

         ID  cayp:order:81b204e8-c404-499e-bc19-d78518a5a3dc                        
       Name  Order for max                                                          
     Status  Succeeded                                                              
 Ordered at  02 Jun 22 16:10 AEST
 Service ID  cayp:service:d939b74d-0070-59a4-a832-36c5c07e657d
 Account ID  cayp:account:58d8e161-9a2b-513a-bd32-28d7e8af1658:testing.com
 Parameters  ┌────────────────────────┐
             │ msg =  Hi, how are you │
             └────────────────────────┘
   Products  ┌────────────────────────────────────────────────────┬─────────┬───────┐
             │ cayp:artifact:017ecae8-3d39-4297-a94f-00ddf9b26611 │ out.png │ 50855 │
             └────────────────────────────────────────────────────┴─────────┴───────┘
```

## Check on Artifacts

To check the details of the artifact created by the previously placed order:

```
% ivcap artifact get cayp:artifact:017ecae8-3d39-4297-a94f-00ddf9b26611

         ID  cayp:artifact:017ecae8-3d39-4297-a94f-00ddf9b26611            
       Name  out.png                                                       
     Status  available                                                     
       Size  50855                                                         
  Mime-type  image/png
 Account ID  cayp:account:58d8e161-9a2b-513a-bd32-28d7e8af1658:testing.com
```

To download the content associated with the artifact.

```
% ivcap artifact download cayp:artifact:017ecae8-3d39-4297-a94f-00ddf9b26611 -o /tmp/out.png
Successfully wrote 50855 bytes to /tmp/out.png
```
