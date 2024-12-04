## ivcap project create

Create a new project

### Synopsis

Create a new project for use on the platform. The project requires a
		name.

```
ivcap project create project_name [flags]
```

### Options

```
  -d, --details string     Details of the project
  -h, --help               help for create
  -n, --name string        Name of project
  -p, --parent_id string   Project ID of the parent of this project
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

* [ivcap project](ivcap_project.md)	 - Create and manage projects 

###### Auto generated by spf13/cobra on 28-Nov-2024