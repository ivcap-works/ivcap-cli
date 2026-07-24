## ivcap project

Manage projects and select the current one

### Options

```
  -h, --help   help for project
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
* [ivcap project create](ivcap_project_create.md)	 - Create a new project
* [ivcap project delete](ivcap_project_delete.md)	 - Delete a project
* [ivcap project get](ivcap_project_get.md)	 - Fetch details about a single project
* [ivcap project grant](ivcap_project_grant.md)	 - Grant project capabilities to a user or service principal
* [ivcap project invitations](ivcap_project_invitations.md)	 - List the pending invitations on a project
* [ivcap project invite](ivcap_project_invite.md)	 - Invite a user to a project
* [ivcap project leave](ivcap_project_leave.md)	 - Leave a project (relinquish your grants)
* [ivcap project list](ivcap_project_list.md)	 - List projects you can access
* [ivcap project members](ivcap_project_members.md)	 - List a project's members and their capabilities
* [ivcap project remove-member](ivcap_project_remove-member.md)	 - Remove a principal from a project entirely (revokes all their capabilities)
* [ivcap project revoke-capability](ivcap_project_revoke-capability.md)	 - Revoke one or more capabilities from a project principal
* [ivcap project use](ivcap_project_use.md)	 - Set the current project for this context (interactive picker if no id given)

