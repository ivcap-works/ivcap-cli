## ivcap context project

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

* [ivcap context](ivcap_context.md)	 - Manage deployment access, projects, and accounts
* [ivcap context project create](ivcap_context_project_create.md)	 - Create a new project
* [ivcap context project delete](ivcap_context_project_delete.md)	 - Delete a project
* [ivcap context project get](ivcap_context_project_get.md)	 - Fetch details about a single project
* [ivcap context project grant](ivcap_context_project_grant.md)	 - Grant project capabilities to a user or service principal
* [ivcap context project invitations](ivcap_context_project_invitations.md)	 - List the pending invitations on a project
* [ivcap context project invite](ivcap_context_project_invite.md)	 - Invite a user to a project
* [ivcap context project leave](ivcap_context_project_leave.md)	 - Leave a project (relinquish your grants)
* [ivcap context project list](ivcap_context_project_list.md)	 - List projects you can access
* [ivcap context project members](ivcap_context_project_members.md)	 - List a project's members and their capabilities
* [ivcap context project remove-member](ivcap_context_project_remove-member.md)	 - Remove a principal from a project entirely (revokes all their capabilities)
* [ivcap context project revoke-capability](ivcap_context_project_revoke-capability.md)	 - Revoke one or more capabilities from a project principal
* [ivcap context project use](ivcap_context_project_use.md)	 - Set the current project for this context (interactive picker if no id given)

