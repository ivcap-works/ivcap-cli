## ivcap

A command line tool to interact with a IVCAP deployment

### Synopsis

A command line tool to to more conveniently interact with the
API exposed by a specific IVCAP deployment.

```
ivcap [flags]
```

### Options

```
      --access-token string   Access token to use for authentication with API server [IVCAP_ACCESS_TOKEN]
      --context string        Context (deployment) to use
      --debug                 Set logging level to DEBUG
  -h, --help                  help for ivcap
      --no-history            Do not store history
  -o, --output string         Set format for displaying output [json, yaml]
      --silent                Do not show any progress information
      --timeout int           Max. number of seconds to wait for completion (default 30)
```

### SEE ALSO

* [ivcap account](ivcap_account.md)	 - Manage accounts you belong to
* [ivcap agent-context](ivcap_agent-context.md)	 - Print embedded agent context guidance (markdown)
* [ivcap artifact](ivcap_artifact.md)	 - Create and manage artifacts
* [ivcap capabilities](ivcap_capabilities.md)	 - List the capabilities grantable on projects and accounts
* [ivcap collection](ivcap_collection.md)	 - Create and manage collections
* [ivcap context](ivcap_context.md)	 - Manage and set access to various IVCAP deployments
* [ivcap datafabric](ivcap_datafabric.md)	 - Query the datafabric and create and manage aspects within
* [ivcap invitation](ivcap_invitation.md)	 - Respond to invitations addressed to you
* [ivcap job](ivcap_job.md)	 - Create and manage jobs
* [ivcap mcp](ivcap_mcp.md)	 - Start an MCP server for accessing all tools on an IVCAP platform
* [ivcap nextflow](ivcap_nextflow.md)	 - Commands for working with Nextflow-based services
* [ivcap package](ivcap_package.md)	 - Push/pull and manage service packages
* [ivcap project](ivcap_project.md)	 - Manage projects and select the current one
* [ivcap queue](ivcap_queue.md)	 - Create and manage queues
* [ivcap secret](ivcap_secret.md)	 - Set and list secrets 
* [ivcap service](ivcap_service.md)	 - Create and manage services
* [ivcap skills](ivcap_skills.md)	 - List and show agent skill docs embedded in this CLI release
* [ivcap whoami](ivcap_whoami.md)	 - Show the currently authenticated identity and accessible accounts/projects

