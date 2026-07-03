## ivcap job events

Stream events for a job

### Synopsis

Stream job-related events in real-time. Events are displayed as they occur.
The service-id is resolved automatically from the job record in the datafabric.

Examples:
  ivcap job events urn:ivcap:job:456
  ivcap job events --max-messages 10 job-id
  ivcap job events --last-event-id abc123 job-id

```
ivcap job events [flags] job-id
```

### Options

```
  -h, --help                   help for events
      --last-event-id string   Last event ID to resume from
      --max-messages int       Maximum number of messages to return (0 = unlimited)
      --max-wait-time int      Max wait time for new events in seconds (default 30)
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

* [ivcap job](ivcap_job.md)	 - Create and manage jobs

