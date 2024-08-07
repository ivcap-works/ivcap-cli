## ivcap queue dequeue

Dequeue messages from a queue

### Synopsis

Dequeue messages from the specified queue. The messages will be written to the specified file in JSON format.

Examples:
  1. Dequeue a message from a queue and save it to a file:
     ivcap queue dequeue urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 messages.json

  2. Dequeue 5 messages from a queue and save them to a file:
     ivcap queue dequeue --limit 5 urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 messages.json


```
ivcap queue dequeue [flags] queue_id file
```

### Options

```
  -h, --help        help for dequeue
  -l, --limit int   Maximum number of messages to dequeue (default 1)
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

* [ivcap queue](ivcap_queue.md)	 - Create and manage queues

###### Auto generated by spf13/cobra on 12-Jun-2024
