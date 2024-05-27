// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/queue"
)

func init() {
	rootCmd.AddCommand(queueCmd)

	// LIST
	listCommand()

	// READ
	readCommand()

	// CREATE
	createCommand()

	// DELETE
	deleteCommand()

	// ENQUEUE
	enqueueCommand()

	// DEQUEUE
	dequeueCommand()
}

var queueCmd = &cobra.Command{
	Use:     "queue",
	Aliases: []string{"q", "queues"},
	Short:   "Create and manage queues",
	Long:    `Queues are used to store messages in a sequential order. You can create, read, update, and delete queues using this command. You can also add and remove messages from queues.`,
}

func listCommand() {
	listQueueCmd := &cobra.Command{
		Use:   "list",
		Short: "List existing queues",
		RunE:  runListQueueCmd,
	}

	queueCmd.AddCommand(listQueueCmd)
	addListFlags(listQueueCmd)
}

func readCommand() {
	readQueueCmd := &cobra.Command{
		Use:     "get [flags] queue_id",
		Aliases: []string{"read"},
		Short:   "Fetch details about a single queue",
		Long: `Fetch details about a single queue. You must provide the ID of the queue you want to read.

An example of reading a queue:

  ivcap queue get urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167`,
		Args: validateReadCommandArgs,
		RunE: runReadQueueCmd,
	}

	queueCmd.AddCommand(readQueueCmd)
}

func createCommand() {
	createQueueCmd := &cobra.Command{
		Use:   "create [flags] name",
		Short: "Create a new queue",
		Long: `Create a new queue with the specified name. Optionally, you can also provide a description and a policy for the queue.

An example of creating a queue with a description:

  ivcap queue create --description "This is a test queue" test_queue`,
		Args: validateCreateQueueArgs,
		RunE: runCreateQueueCmd,
	}

	queueCmd.AddCommand(createQueueCmd)
	createQueueCmd.Flags().StringP("description", "d", "", "Description of the queue")
	addPolicyFlag(createQueueCmd)
}

func deleteCommand() {
	deleteQueueCmd := &cobra.Command{
		Use:     "delete [flags] queue_id",
		Aliases: []string{"remove"},
		Short:   "Delete a queue",
		Long: `Delete a queue with the specified ID. This action is irreversible. All messages in the queue will be lost.
An example of deleting a queue:

  ivcap queue delete test_queue`,
		Args: validateDeleteCommandArgs,
		RunE: runDeleteQueueCmd,
	}
	queueCmd.AddCommand(deleteQueueCmd)
}

func enqueueCommand() {
	enqueueCmd := &cobra.Command{
		Use:   "enqueue [flags] schema queue_id message",
		Short: "Enqueue a message to a queue",
		Long: `Enqueue a message to the specified queue. The message content must be in JSON format. You must also provide the schema of the message content.

An example of enqueuing a message to a queue:

  ivcap queue enqueue urn:ivcap:schema:queue:message.1 urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 "{\"temperature\": \"21\", \"location\": \"Buoy101\", \"timestamp\": \"2024-05-20T14:30:00Z\"}"`,
		Args: validateEnqueueArgs,
		RunE: runEnqueueCmd,
	}

	queueCmd.AddCommand(enqueueCmd)
}

func dequeueCommand() {
	dequeueCmd := &cobra.Command{
		Use:   "dequeue [flags] queue_id",
		Short: "Dequeue messages from a queue",
		Long: `Dequeue messages from the specified queue. By default, only one message is dequeued. You can specify the number of messages to dequeue using the --limit flag. 

An example of dequeuing a message from a queue:

  ivcap queue dequeue --limit 5 urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167`,
		Args: validateDequeueArgs,
		RunE: runDequeueCmd,
	}

	queueCmd.AddCommand(dequeueCmd)
	dequeueCmd.Flags().IntP("limit", "l", 1, "Maximum number of messages to dequeue")
}

func runListQueueCmd(cmd *cobra.Command, args []string) error {
	req := createListRequest()

	err := printResponseBody(
		func() (a.Payload, error) {
			return sdk.ListQueuesRaw(context.Background(), req, CreateAdapter(true), logger)
		},
		func() (*api.ListResponseBody, error) {
			return sdk.ListQueues(context.Background(), req, CreateAdapter(true), logger)
		},
		func(res *api.ListResponseBody) {
			printListResponse(res)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to list queues: %w", err)
	}

	return nil
}

func validateReadCommandArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide the ID of the queue to read. Example: ivcap queue %s urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167", cmd.Name())
	}
	return cobra.ExactArgs(1)(cmd, args)
}

func runReadQueueCmd(cmd *cobra.Command, args []string) error {
	recordID := GetHistory(args[0])
	req := &sdk.ReadQueueRequest{Id: GetHistory(recordID)}

	err := printResponseBody(
		func() (a.Payload, error) {
			return sdk.ReadQueueRaw(context.Background(), req, CreateAdapter(true), logger)
		},
		func() (*api.ReadResponseBody, error) {
			return sdk.ReadQueue(context.Background(), req, CreateAdapter(true), logger)
		},
		func(res *api.ReadResponseBody) {
			printReadResponse(res)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to read queue: %w", err)
	}

	return nil
}

func validateCreateQueueArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide a name for the queue. Example: ivcap queue %s my-queue-name", cmd.Name())
	}
	return cobra.ExactArgs(1)(cmd, args)
}

func runCreateQueueCmd(cmd *cobra.Command, args []string) error {
	name := args[0]
	req := &api.CreateRequestBody{
		Name: name,
	}

	description, _ := cmd.Flags().GetString("description")
	if description != "" {
		req.Description = &description
	}

	policy, _ := cmd.Flags().GetString("policy")
	if policy != "" {
		req.Policy = &policy
	}

	err := printResponseBody(
		func() (a.Payload, error) {
			return sdk.CreateQueueRaw(context.Background(), req, CreateAdapter(true), logger)
		},
		func() (*api.CreateResponseBody, error) {
			return sdk.CreateQueue(context.Background(), req, CreateAdapter(true), logger)
		},
		func(res *api.CreateResponseBody) {
			printCreateResponse(res)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}

	return nil
}

func validateEnqueueArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("please provide the schema, queue ID, and message content to enqueue a message. Example: ivcap queue %s urn:ivcap:schema:queue:message.1 urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 '{\"temperature\": \"21\", \"location\": \"Buoy101\", \"timestamp\": \"2024-05-20T14:30:00Z\"}'", cmd.Name())
	}
	return cobra.ExactArgs(3)(cmd, args)
}

func runEnqueueCmd(cmd *cobra.Command, args []string) error {
	schema, queueID, message := args[0], GetHistory(args[1]), args[2]
	req := &sdk.ReadQueueRequest{Id: GetHistory(queueID)}

	err := printResponseBody(
		func() (a.Payload, error) {
			return sdk.EnqueueRaw(context.Background(), req, schema, message, CreateAdapter(true), logger)
		},
		func() (*api.EnqueueResponseBody, error) {
			return sdk.Enqueue(context.Background(), req, schema, message, CreateAdapter(true), logger)
		},
		func(res *api.EnqueueResponseBody) {
			fmt.Printf("Message enqueued to queue %s with ID %s\n", queueID, *res.ID)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	return nil
}

func validateDequeueArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide the ID of the queue to dequeue messages. Example: ivcap queue %s urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167", cmd.Name())
	}
	return cobra.ExactArgs(1)(cmd, args)
}

func runDequeueCmd(cmd *cobra.Command, args []string) error {
	recordID := GetHistory(args[0])
	req := &sdk.ReadQueueRequest{Id: GetHistory(recordID)}

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		limit = 1 // Default value if the flag is not set or invalid
	}

	err = printResponseBody(
		func() (a.Payload, error) {
			return sdk.DequeueRaw(context.Background(), req, limit, CreateAdapter(true), logger)
		},
		func() (*api.DequeueResponseBody, error) {
			return sdk.Dequeue(context.Background(), req, limit, CreateAdapter(true), logger)
		},
		func(res *api.DequeueResponseBody) {
			printDequeueResponse(res)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to dequeue messages: %w", err)
	}

	return nil
}

func validateDeleteCommandArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide the ID of the queue to delete. Example: ivcap queue %s urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167", cmd.Name())
	}
	return cobra.ExactArgs(1)(cmd, args)
}

func runDeleteQueueCmd(cmd *cobra.Command, args []string) error {
	recordID := GetHistory(args[0])
	req := &sdk.ReadQueueRequest{Id: GetHistory(recordID)}

	res, err := sdk.DeleteQueueRaw(context.Background(), req, CreateAdapter(true), logger)
	if err != nil {
		return fmt.Errorf("failed to delete queue: %w", err)
	}

	switch outputFormat {
	case "json", "yaml":
		return a.ReplyPrinter(res, outputFormat == "yaml")
	default:
		fmt.Printf("Queue %s deleted\n", recordID)
	}
	return nil
}

func printResponseBody[ResponseType any](
	rawRequestFunc func() (a.Payload, error),
	requestFunc func() (*ResponseType, error),
	printFunc func(*ResponseType),
) error {
	switch outputFormat {
	case "json", "yaml":
		if res, err := rawRequestFunc(); err == nil {
			return a.ReplyPrinter(res, outputFormat == "yaml")
		} else {
			return fmt.Errorf("failed to list queues: %w", err)
		}
	default:
		if res, err := requestFunc(); err == nil {
			printFunc(res)
		} else {
			return fmt.Errorf("failed to list queues: %w", err)
		}
	}
	return nil
}

func printDequeueResponse(response *api.DequeueResponseBody) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Content Type", "Schema", "Content"})

	rows := make([]table.Row, len(response.Messages))
	for i, msg := range response.Messages {
		content := marshalContent(msg.Content)
		rows[i] = table.Row{safeString(msg.ID), safeString(msg.ContentType), safeString(msg.Schema), safeTruncString(&content)}
	}

	t.AppendRows(rows)
	t.Render()
}

func marshalContent(content any) string {
	jsonBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Sprintf("%v", content)
	}
	return string(jsonBytes)
}

func printListResponse(list *api.ListResponseBody) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Account"})

	rows := make([]table.Row, len(list.Items))
	for i, o := range list.Items {
		rows[i] = table.Row{MakeHistory(o.ID), safeTruncString(o.Name), safeString(o.Account)}
	}

	rows = addNextPageRow(findNextQueuePage(list.Links), rows)
	t.AppendRows(rows)
	t.Render()
}

func findNextQueuePage(links []*api.LinkTResponseBody) *string {
	if links == nil {
		return nil
	}
	for _, l := range links {
		if l.Rel != nil && *l.Rel == "next" {
			return l.Href
		}
	}
	return nil
}

func printReadResponse(queue *api.ReadResponseBody) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
	})

	var totalMessagesStr string
	if queue.TotalMessages != nil {
		totalMessagesStr = fmt.Sprintf("%d", *queue.TotalMessages)
	}

	var bytesStr string
	if queue.Bytes != nil {
		bytesStr = fmt.Sprintf("%d", *queue.Bytes)
	}

	var consumerCountStr string
	if queue.ConsumerCount != nil {
		consumerCountStr = fmt.Sprintf("%d", *queue.ConsumerCount)
	}

	tw.AppendRows([]table.Row{
		{"ID", fmt.Sprintf("%s (%s)", *queue.ID, MakeHistory(queue.ID))},
		{"Name", safeString(queue.Name)},
		{"Description", safeString(queue.Description)},
		{"Created At", safeString(queue.CreatedAt)},
		{"Total Messages", safeString(&totalMessagesStr)},
		{"Bytes", safeString(&bytesStr)},
		{"Consumer Count", safeString(&consumerCountStr)},
		{"First ID", safeString(queue.FirstID)},
		{"Last ID", safeString(queue.LastID)},
		{"First Time", safeString(queue.FirstTime)},
		{"Last Time", safeString(queue.LastTime)},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func printCreateResponse(queue *api.CreateResponseBody) {
	fmt.Printf("Queue %s created\n", *queue.ID)
}
