// Copyright 2023-2025 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
	"math"
	"strings"
	"time"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	aspect "github.com/ivcap-works/ivcap-core-api/http/aspect"
	log "go.uber.org/zap"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

const JOB_SCHEMA = "urn:ivcap:schema:job.2"

// maxResultLines is the maximum number of text lines to show for a job result
// in the human-readable table output.  Long results are truncated with a count
// of the omitted lines so that the terminal output stays manageable.
const maxResultLines = 10

// maxResultLineWidth is the maximum number of characters per line when
// displaying job result content.  Lines longer than this are trimmed with "..."
const maxResultLineWidth = 80

const CREATE_FROM_ASPECT = sdk.CreateFromAspectTemplate

func init() {
	rootCmd.AddCommand(jobCmd)

	// LIST
	jobCmd.AddCommand(listJobCmd)
	listJobCmd.Flags().StringVarP(&jobsJsonFilter, "content-path", "c", "", "json path filter on jobs's content ('$.images[*] ? (@.size > 10000)')")
	addListFlags(listJobCmd)

	// READ
	jobCmd.AddCommand(readJobCmd)

	// CREATE
	jobCmd.AddCommand(createJobCmd)
	addFileFlag(createJobCmd, "Path to job description file")
	addInputFormatFlag(createJobCmd)
	createJobCmd.Flags().StringVarP(&aspectURN, "aspect", "a", "", "URN of aspect containing job parameters")
	createJobCmd.Flags().BoolVar(&watchFlag, "watch", false, "if set, watch the job until it is finished")
	createJobCmd.Flags().BoolVar(&streamFlag, "stream", false, "if set, print job related events to stdout")

	// EVENTS
	jobCmd.AddCommand(eventsJobCmd)
	eventsJobCmd.Flags().IntVar(&maxMessages, "max-messages", 0, "Maximum number of messages to return (0 = unlimited)")
	eventsJobCmd.Flags().IntVar(&maxWaitTime, "max-wait-time", 30, "Max wait time for new events in seconds")
	eventsJobCmd.Flags().StringVar(&lastEventID, "last-event-id", "", "Last event ID to resume from")
}

var (
	jobsJsonFilter string
	aspectURN      string
	watchFlag      bool
	streamFlag     bool
	maxMessages    int
	maxWaitTime    int
	lastEventID    string
)

var (
	jobCmd = &cobra.Command{
		Use:     "job",
		Aliases: []string{"js", "jobs"},
		Short:   "Create and manage jobs",
	}

	listJobCmd = &cobra.Command{
		Use:   "list",
		Short: "List existing jobs",

		RunE: func(cmd *cobra.Command, args []string) error {
			lr := createListRequest()
			if lr.OrderBy == nil {
				rb := "requested-at"
				lr.OrderBy = &rb
			}
			selector := sdk.AspectSelector{
				SchemaPrefix:   JOB_SCHEMA,
				ListRequest:    *lr,
				IncludeContent: true,
			}
			if jobsJsonFilter != "" {
				selector.JsonFilter = &jobsJsonFilter
			}
			ctxt := context.Background()
			if list, res, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					printJobListTable(list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	readJobCmd = &cobra.Command{
		Use:     "get [flags] job_id",
		Aliases: []string{"read", "g"},
		Short:   "Fetch details about a single job",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := GetHistory(args[0])
			ctxt := context.Background()
			return readDisplayJob(ctxt, recordID)
		},
	}

	eventsJobCmd = &cobra.Command{
		Use:   "events [flags] service-id job-id",
		Short: "Stream events for a job",
		Long: `Stream job-related events in real-time. Events are displayed as they occur.

Examples:
  ivcap job events urn:ivcap:service:123 urn:ivcap:job:456
  ivcap job events --max-messages 10 service-id job-id
  ivcap job events --last-event-id abc123 service-id job-id`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceID := GetHistory(args[0])
			jobID := GetHistory(args[1])
			ctxt := context.Background()

			var lastID *string
			if lastEventID != "" {
				lastID = &lastEventID
			}
			return streamJobEvents(ctxt, serviceID, jobID, lastID, maxMessages)
		},
	}

	createJobCmd = &cobra.Command{
		Use:   "create [flags] service-id -f job-input|- -a aspect-urn --watch --stream",
		Short: "Create a new job",
		Long: `Create a new job by executing the service 'service-id' with the
input paramters defined in either a provided (json) file or a reference
to an aspect containing the parameter definitions. If the job definition is
provided through 'stdin' use '-' as the file name and also include the --format flag`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()

			if fileName == "" && aspectURN == "" {
				cobra.CheckErr("Missing parameter file '-f job-file|-' or '-a aspectURN'")
			}

			serviceID := GetHistory(args[0])

			var pyld a.Payload
			if fileName != "" {
				if pyld, err = payloadFromFile(fileName, inputFormat); err != nil {
					cobra.CheckErr(fmt.Sprintf("While reading job file '%s' - %v", fileName, err))
				}
			}
			if aspectURN != "" {
				j := fmt.Sprintf(CREATE_FROM_ASPECT, aspectURN, serviceID)
				if pyld, err = a.LoadPayloadFromBytes([]byte(j), false); err != nil {
					cobra.CheckErr(fmt.Sprintf("While reading job file '%s' - %v", fileName, err))
				}
			}
			res, jobCreate, err := sdk.CreateServiceJobRaw(ctxt, serviceID, pyld, 0, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			if jobCreate != nil {
				return waitForResult(ctxt, jobCreate, serviceID)
			}
			reply, err := res.AsObject()
			if err != nil {
				return err
			}
			jobID, ok := reply["job-id"].(string)
			if !ok {
				cobra.CheckErr("Cannot find job ID in response")
			}
			return readDisplayJob(ctxt, jobID) // a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}
)

func waitForResult(
	ctxt context.Context,
	jobCreate *sdk.JobCreateT,
	serviceID string,
) error {
	// jobCreate.ServiceID = serviceID
	if streamFlag {
		return streamJobResults(ctxt, jobCreate)
	}
	wait := 2
	if !watchFlag {
		wait = int(math.Min(jobCreate.RetryLater, float64(timeout)))
	}
	logger.Info("Job created", log.String("job-id", jobCreate.JobID), log.Int("waiting [sec]", wait))

	jobID := jobCreate.JobID
	maxCheck := 1
	if watchFlag {
		maxCheck = 99 // should really define that in t terms of max. wait
	}
	job, pyld, err := watchJob(ctxt, jobID, maxCheck, wait)
	if err != nil {
		return err
	}
	return displayJob(job, pyld, nil, nil)
}

func watchJob(ctxt context.Context, jobID string, maxChecks int, wait int) (*sdk.JobReadResponseBody, a.Payload, error) {
	done := false
	tries := 0
	for !done {
		time.Sleep(time.Duration(wait) * time.Second)
		job, pyld, _, _, err := readJob(ctxt, jobID)
		if err != nil {
			return nil, nil, err
		}
		status := "?"
		if job.Status != nil {
			status = *job.Status
		}
		tries += 1
		done = tries >= maxChecks || (status != "?" && status != "scheduled" && status != "executing")
		if done {
			return job, pyld, nil
		}
	}
	return nil, nil, fmt.Errorf("timed out waiting for job to finish")
}

func streamJobResults(ctxt context.Context, jobCreate *sdk.JobCreateT) error {
	if err := streamJobEvents(ctxt, jobCreate.ServiceID, jobCreate.JobID, nil, 0); err != nil {
		cobra.CheckErr(fmt.Sprintf("While watching events for job '%s' - %s", jobCreate.JobID, err))
	}
	return readDisplayJob(ctxt, jobCreate.JobID)
}

func readDisplayJob(ctxt context.Context, jobID string) error {
	job, pyld, jobResultAspect, nextflowResultAspect, err := readJob(ctxt, jobID)
	if err != nil {
		return err
	}
	return displayJob(job, pyld, jobResultAspect, nextflowResultAspect)
}

func displayJob(job *sdk.JobReadResponseBody, pyld a.Payload, jobResultAspect map[string]any, nextflowResultAspect map[string]any) error {
	switch outputFormat {
	case "json", "yaml":
		return a.ReplyPrinter(pyld, outputFormat == "yaml")
	default:
		printJob(job, jobResultAspect, nextflowResultAspect, false)
	}
	return nil
}

func readJob(ctxt context.Context, jobID string) (*sdk.JobReadResponseBody, a.Payload, map[string]any, map[string]any, error) {
	selector := sdk.AspectSelector{
		Entity:         jobID,
		SchemaPrefix:   JOB_SCHEMA,
		IncludeContent: true,
	}
	var serviceId string
	if list, _, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
		if len(list.Items) != 1 {
			cobra.CheckErr("Cannot find job")
		}
		c := list.Items[0].Content.(map[string]any)
		if s, ok := c["service-id"].(string); ok {
			serviceId = s
		} else {
			cobra.CheckErr("Cannot find 'service-id' for this job")
		}
	} else {
		return nil, nil, nil, nil, err
	}
	req := &sdk.ReadServiceJobRequest{ServiceId: serviceId, JobId: jobID}
	job, pyld, err := sdk.ReadServiceJob(context.Background(), req, CreateAdapter(true), logger)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Try to fetch job.result.1 aspect (execution phase information)
	jobResultAspect := readJobResultAspect(ctxt, jobID)

	// Try to fetch nextflow.result.1 aspect (detailed results after completion)
	nextflowResultAspect := readNextflowResultAspectFromJob(ctxt, jobID)

	return job, pyld, jobResultAspect, nextflowResultAspect, nil
}

// readJobResultAspect reads the job.result.1 aspect for a job (execution phase information)
func readJobResultAspect(ctxt context.Context, jobID string) map[string]any {
	selector := sdk.AspectSelector{
		Entity:         jobID,
		SchemaPrefix:   "urn:ivcap:schema:job.result.1",
		IncludeContent: true,
	}

	list, _, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger)
	if err != nil {
		return nil
	}

	if len(list.Items) == 0 {
		return nil
	}

	// Get the first (and should be only) item
	if content, ok := list.Items[0].Content.(map[string]any); ok {
		return content
	}

	return nil
}

// readNextflowResultAspectFromJob reads the nextflow.result.1 aspect for a job
func readNextflowResultAspectFromJob(ctxt context.Context, jobID string) map[string]any {
	selector := sdk.AspectSelector{
		Entity:         jobID,
		SchemaPrefix:   "urn:ivcap:schema:nextflow.result.1",
		IncludeContent: true,
	}

	list, _, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger)
	if err != nil {
		return nil
	}

	if len(list.Items) == 0 {
		return nil
	}

	// Get the first (and should be only) item
	if content, ok := list.Items[0].Content.(map[string]any); ok {
		return content
	}

	return nil
}

// truncateResultContent formats a text blob for display in the Result table row.
// It limits to maxResultLines lines and maxResultLineWidth characters per line.
// Truncation indicators are always placed on their own line.
func truncateResultContent(text string) string {
	lines := strings.Split(text, "\n")

	// Truncate each individual line to maxResultLineWidth characters
	for i, line := range lines {
		if len(line) > maxResultLineWidth {
			lines[i] = line[:maxResultLineWidth] + "..."
		}
	}

	// Cap total line count; put the "more lines" notice on its own new line
	if len(lines) > maxResultLines {
		omitted := len(lines) - maxResultLines
		lines = append(lines[:maxResultLines], fmt.Sprintf("... (%d more lines)", omitted))
	}

	return strings.Join(lines, "\n")
}

func printJobListTable(list *aspect.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"ID", "Service", "Status", "Requested At"})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(list.Items))
	for i, p := range list.Items {
		c := p.Content.(map[string]any)
		id := c["id"].(string)

		service := "???"
		if s, ok := c["service-name"].(string); ok {
			service = s
		} else if s2, ok2 := c["service-id"].(string); ok2 {
			service = s2
		}

		status := "unknown"
		if s, ok := c["status"].(string); ok {
			status = s
		}
		requestedAt := ""
		if s, ok := c["requested-at"].(string); ok {
			requestedAt = safeDate(&s, true)
		}

		rows[i] = table.Row{MakeHistory(&id), service, status, requestedAt}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		// {Number: 2, WidthMax: 80},
	})

	p := []table.Row{}
	if list.AtTime != nil {
		p = append(p, table.Row{"At Time", safeDate(list.AtTime, false)})
	}
	p = append(p, table.Row{"Jobs", tw2.Render()})
	p = addNextPageRow(findNextAspectPage(list.Links), p)
	tw.AppendRows(p)

	fmt.Printf("\n%s\n\n", tw.Render())
}

func printJob(job *sdk.JobReadResponseBody, jobResultAspect map[string]any, nextflowResultAspect map[string]any, wide bool) {

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false

	rows := []table.Row{}

	// Name at the top if available
	if job.Name != nil {
		rows = append(rows, table.Row{"Name", safeString(job.Name)})
	}

	// IVCAP Job Status
	rows = append(rows, table.Row{"IVCAP Status", safeString(job.Status)})

	// Job Phase Information (from job.result.1 aspect)
	if jobResultAspect != nil {
		if phase, ok := jobResultAspect["phase"].(string); ok && phase != "" {
			rows = append(rows, table.Row{"Phase", phase})
		}
	}

	// Result - check nextflow.result.1 aspect first (finished Nextflow jobs)
	// If available, display the status and process results from the aspect
	if len(nextflowResultAspect) > 0 {
		// Display status from nextflow result aspect
		if status, ok := nextflowResultAspect["status"].(string); ok && status != "" {
			rows = append(rows, table.Row{"Nxf Status", status})
		}

		// Display log artifact if available
		if logURN, ok := nextflowResultAspect["log_urn"].(string); ok && logURN != "" {
			logDisplay := fmt.Sprintf("%s (%s)", logURN, MakeHistory(&logURN))
			rows = append(rows, table.Row{"Log", logDisplay})
		}

		// Display process results if available
		if results, ok := nextflowResultAspect["results"].(map[string]interface{}); ok && len(results) > 0 {
			rows = append(rows, table.Row{"", ""})
			rows = append(rows, table.Row{"Processes", ""})
			for procName, procURN := range results {
				if urn, ok := procURN.(string); ok && urn != "" {
					procDisplay := fmt.Sprintf("%s (%s)", urn, MakeHistory(&urn))
					rows = append(rows, table.Row{"  " + procName, procDisplay})
				}
			}
		}
	} else {
		// Show ResultContentUrn (if present) followed by the result content
		// JSON truncated to maxResultLines lines so the terminal stays readable.
		resultDisplay := "-"
		if job.ResultContentUrn != nil {
			urn := *job.ResultContentUrn
			resultDisplay = fmt.Sprintf("%s (%s)", urn, MakeHistory(&urn))
			if job.ResultContent != nil {
				if jsonBytes, err := json.MarshalIndent(job.ResultContent, "", "  "); err == nil {
					resultDisplay += "\n" + truncateResultContent(string(jsonBytes))
				}
			}
		} else if job.ResultContent != nil {
			// Try to parse ResultContent as JSON and extract results_artifact_urn
			if contentMap, ok := job.ResultContent.(map[string]interface{}); ok {
				if artifactUrn, ok := contentMap["results_artifact_urn"].(string); ok && artifactUrn != "" {
					resultDisplay = fmt.Sprintf("%s (%s)", artifactUrn, MakeHistory(&artifactUrn))
				} else {
					// No well-known URN field – render the raw content truncated
					if jsonBytes, err := json.MarshalIndent(job.ResultContent, "", "  "); err == nil {
						resultDisplay = truncateResultContent(string(jsonBytes))
					}
				}
			}
		}
		rows = append(rows, table.Row{"Result", resultDisplay})
	}

	// Empty line separator
	rows = append(rows, table.Row{"", ""})

	// ID
	id := fmt.Sprintf("%s (%s)", *job.ID, MakeHistory(job.ID))
	rows = append(rows, table.Row{"ID", id})

	// Started At
	rows = append(rows, table.Row{"Started At", safeDate(job.StartedAt, false)})

	// Finished At (if available)
	if job.FinishedAt != nil {
		rows = append(rows,
			table.Row{"Finished At", safeDate(job.FinishedAt, false)},
		)
	}

	// Service
	var service string
	if job.Service != nil {
		service = fmt.Sprintf("%s (%s)", *job.Service, MakeHistory(job.Service))
	}
	rows = append(rows, table.Row{"Service", service})

	// Policy and Account
	rows = append(rows,
		table.Row{"Policy", safeString(job.Policy)},
		table.Row{"Account", safeString(job.Account)},
	)

	tw.AppendRows(rows)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}
