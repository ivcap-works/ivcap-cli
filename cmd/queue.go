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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	longDesc := `Enqueue a message from a file to the specified queue. The message must be in JSON format.

An example of enqueuing a message to a queue:

  ivcap queue enqueue urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 urn:ivcap:schema:queue:message.1 message.json`

	args := map[string]string{
		"queue_id": "The unique identifier of the queue from which to dequeue messages.",
		"schema":   "The schema of the message to enqueue.",
		"file":     "The file containing the message to enqueue. If the message is provided through 'stdin' use '-' as the file name.",
	}

	enqueueCmd := &cobra.Command{
		Use:   "enqueue [flags] queue_id schema file",
		Short: "Enqueue a message to a queue",
		Long:  longDesc,
		Args:  validateEnqueueArgs,
		RunE:  runEnqueueCmd,
	}

	enqueueCmd.SetHelpTemplate(helpTemplate(args))
	queueCmd.AddCommand(enqueueCmd)
}

func dequeueCommand() {
	longDesc := `Dequeue messages from the specified queue. The messages will be written to the specified file in JSON format.

An example of dequeuing messages from a queue:

    ivcap queue dequeue urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 messages.json`

	args := map[string]string{
		"queue_id": "The unique identifier of the queue from which to dequeue messages.",
		"file":     "The file to write the dequeued messages to.",
	}

	dequeueCmd := &cobra.Command{
		Use:   "dequeue [flags] queue_id file",
		Short: "Dequeue messages from a queue",
		Long:  longDesc,
		Args:  validateDequeueArgs,
		RunE:  runDequeueCmd,
	}

	dequeueCmd.Flags().IntP("limit", "l", 1, "Maximum number of messages to dequeue")
	dequeueCmd.SetHelpTemplate(helpTemplate(args))
	queueCmd.AddCommand(dequeueCmd)
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
	errMsg := "please provide the ID of the queue, the schema, and the file containing the message to enqueue."
	exampleUsage := fmt.Sprintf("Example: ivcap queue %s urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 urn:ivcap:schema:queue:message.1 message.json", cmd.Name())

	switch len(args) {
	case 1:
		switch {
		case strings.Contains(args[0], "urn:ivcap:queue:"):
			errMsg = "You have provided the queue ID. Please also provide the schema and the file containing the message to enqueue."
		case strings.Contains(args[0], "urn:ivcap:schema:"):
			errMsg = "You have provided the schema. Please also provide the queue ID and the file containing the message to enqueue."
		default:
			errMsg = "You have provided the file name. Please also provide the queue ID and the schema."
		}
	case 2:
		switch {
		case strings.Contains(args[0], "urn:ivcap:queue:") && strings.Contains(args[1], "urn:ivcap:schema:"):
			errMsg = "You have provided the queue ID and the schema. Please also provide the file containing the message to enqueue."
		case strings.Contains(args[0], "urn:ivcap:queue:"):
			errMsg = "You have provided the queue ID and the file name. Please also provide the schema."
		default:
			errMsg = "You have provided the schema and the file name. Please also provide the queue ID."
		}
	}

	if len(args) < 3 {
		return fmt.Errorf("%s\n\n%s", errMsg, exampleUsage)
	}

	return cobra.ExactArgs(3)(cmd, args)
}

func runEnqueueCmd(cmd *cobra.Command, args []string) error {
	queueID, schema, filepath := GetHistory(args[0]), args[1], args[2]
	req := &sdk.ReadQueueRequest{Id: GetHistory(queueID)}

	payload, err := payloadFromFile(filepath, "json")
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("While reading service file '%s' - %s", fileName, err))
	}

	message := string(payload.AsBytes())

	err = printResponseBody(
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
	if len(args) < 2 {
		errMsg := "please provide the ID of the queue to dequeue messages from and the file to write the messages to."
		exampleUsage := fmt.Sprintf("Example: ivcap queue %s urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167 messages.json", cmd.Name())

		if len(args) == 1 {
			if strings.Contains(args[0], "urn:ivcap:queue:") {
				errMsg = "You have provided the queue ID. Please also provide the file to write the messages to."
			} else {
				errMsg = "You have provided the file name. Please also provide the ID of the queue to dequeue messages from."
			}
		}

		return fmt.Errorf("%s\n\n%s", errMsg, exampleUsage)
	}
	return cobra.ExactArgs(2)(cmd, args)
}

func runDequeueCmd(cmd *cobra.Command, args []string) error {
	recordID := GetHistory(args[0])
	req := &sdk.ReadQueueRequest{Id: GetHistory(recordID)}

	filePath := args[1]

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		limit = 1 // Default value if the flag is not set or invalid
	}

	payload, err := sdk.DequeueRaw(context.Background(), req, limit, CreateAdapter(true), logger)
	if err != nil {
		return fmt.Errorf("failed to dequeue messages: %w", err)
	}

	err = printDequeueResponse(payload, filePath)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
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
			return fmt.Errorf("failed to print response: %w", err)
		}
	default:
		if res, err := requestFunc(); err == nil {
			printFunc(res)
		} else {
			return fmt.Errorf("failed to print response: %w", err)
		}
	}
	return nil
}

func printDequeueResponse(response a.Payload, filePath string) error {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, response.AsBytes(), "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	file, err := safeOpen(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = file.Write(prettyJSON.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func safeOpen(filePath string) (*os.File, error) {
	// Clean the filePath to prevent path traversal
	cleanPath := filepath.Clean(filePath)

	// If the path is not absolute, make it relative to the current directory
	if !filepath.IsAbs(cleanPath) {
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("unable to get current directory: %v", err)
		}
		cleanPath = filepath.Join(currentDir, cleanPath)
	}

	// Create the file
	return os.Create(cleanPath)
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

func getMaxLen(args map[string]string) int {
	maxArgLen := 0
	for arg := range args {
		if len(arg) > maxArgLen {
			maxArgLen = len(arg)
		}
	}
	return maxArgLen
}

func helpTemplate(args map[string]string) string {
	var argsStr strings.Builder

	// Write the arguments and descriptions to the string builder
	maxArgLen := getMaxLen(args)
	for arg, desc := range args {
		padding := strings.Repeat(" ", maxArgLen-len(arg))
		argsStr.WriteString(fmt.Sprintf("  %s%s  %s\n", arg, padding, desc))
	}

	return fmt.Sprintf(`{{with .Long}}{{. | trimTrailingWhitespaces}}{{end}}

Usage:
  {{.UseLine}}

Arguments:
%s
{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}`, argsStr.String())
}
