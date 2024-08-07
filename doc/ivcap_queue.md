## ivcap queue

Create and manage queues

### Synopsis

Queues are used to store messages in a sequential order. You can create, read, update, and delete queues using this command. You can also add and remove messages from queues.

### Options

```
  -h, --help   help for queue
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
* [ivcap queue create](ivcap_queue_create.md)	 - Create a new queue
* [ivcap queue delete](ivcap_queue_delete.md)	 - Delete a queue
* [ivcap queue dequeue](ivcap_queue_dequeue.md)	 - Dequeue messages from a queue
* [ivcap queue enqueue](ivcap_queue_enqueue.md)	 - Enqueue a message to a queue
* [ivcap queue get](ivcap_queue_get.md)	 - Fetch details about a single queue
* [ivcap queue list](ivcap_queue_list.md)	 - List existing queues

###### Auto generated by spf13/cobra on 12-Jun-2024
