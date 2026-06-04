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

// jobResultPreviewLines is the maximum number of JSON lines shown when
// previewing a job's output content (out-content-urn).
const jobResultPreviewLines = 10

const CREATE_FROM_ASPECT = sdk.CreateFromAspectTemplate

// jobOutContent holds pre-fetched output content info for inline display.
type jobOutContent struct {
	urn     string // out-content-urn from job result aspect
	preview string // first jobResultPreviewLines lines of JSON, or empty
}

// fetchJobOutContent inspects the jobResultAspect for "out-content-type" /
// "out-content-urn". When the URN is present and the content type is
// "application/json" it fetches the aspect and returns the first
// jobResultPreviewLines lines of the pretty-printed JSON as preview.
func fetchJobOutContent(ctxt context.Context, jobResultAspect map[string]any) *jobOutContent {
	outContentUrn, _ := jobResultAspect["out-content-urn"].(string)
	if outContentUrn == "" {
		return nil
	}
	outContentType, _ := jobResultAspect["out-content-type"].(string)

	result := &jobOutContent{urn: outContentUrn}

	if outContentType == "application/json" {
		asp, err := sdk.GetAspect(ctxt, outContentUrn, CreateAdapter(true), logger)
		if err == nil && asp != nil && asp.Content != nil {
			jsonBytes, err := json.MarshalIndent(asp.Content, "", "  ")
			if err == nil {
				lines := strings.Split(string(jsonBytes), "\n")
				if len(lines) > jobResultPreviewLines {
					result.preview = strings.Join(lines[:jobResultPreviewLines], "\n") + "\n..."
				} else {
					result.preview = strings.Join(lines, "\n")
				}
			}
		}
	}

	return result
}

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
					printJobListTable(list)
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
		Use:   "events [flags] job-id",
		Short: "Stream events for a job",
		Long: `Stream job-related events in real-time. Events are displayed as they occur.
The service ID is automatically resolved from the job ID.

Examples:
  ivcap job events urn:ivcap:job:456
  ivcap job events --max-messages 10 job-id
  ivcap job events --last-event-id abc123 job-id`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := GetHistory(args[0])
			ctxt := context.Background()

			serviceID, err := lookupServiceIDForJob(ctxt, jobID)
			if err != nil {
				return err
			}

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
	jobCreate.ServiceID = serviceID
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
	return displayJob(job, pyld, nil, nil, nil)
}

func watchJob(ctxt context.Context, jobID string, maxChecks int, wait int) (*sdk.JobReadResponseBody, a.Payload, error) {
	done := false
	tries := 0
	for !done {
		time.Sleep(time.Duration(wait) * time.Second)
		job, pyld, _, _, _, err := readJob(ctxt, jobID)
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
	// Use the serviceID we already have from the job creation response — avoids a
	// lookupServiceIDForJob round-trip that can fail if the aspects endpoint is not
	// reachable or the job aspect has not been persisted yet.
	job, pyld, jobResultAspect, nextflowResultAspect, outContent, err := readJobWithServiceID(ctxt, jobCreate.ServiceID, jobCreate.JobID)
	if err != nil {
		return err
	}
	return displayJob(job, pyld, jobResultAspect, nextflowResultAspect, outContent)
}

func readDisplayJob(ctxt context.Context, jobID string) error {
	job, pyld, jobResultAspect, nextflowResultAspect, outContent, err := readJob(ctxt, jobID)
	if err != nil {
		return err
	}
	return displayJob(job, pyld, jobResultAspect, nextflowResultAspect, outContent)
}

func displayJob(job *sdk.JobReadResponseBody, pyld a.Payload, jobResultAspect map[string]any, nextflowResultAspect map[string]any, outContent *jobOutContent) error {
	switch outputFormat {
	case "json", "yaml":
		return a.ReplyPrinter(pyld, outputFormat == "yaml")
	default:
		printJob(job, jobResultAspect, nextflowResultAspect, outContent, false)
	}
	return nil
}

// lookupServiceIDForJob resolves the service ID for a given job ID by querying the job aspect.
func lookupServiceIDForJob(ctxt context.Context, jobID string) (string, error) {
	selector := sdk.AspectSelector{
		Entity:         jobID,
		SchemaPrefix:   JOB_SCHEMA,
		IncludeContent: true,
	}
	list, _, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger)
	if err != nil {
		return "", err
	}
	if len(list.Items) != 1 {
		return "", fmt.Errorf("cannot find job '%s'", jobID)
	}
	c := list.Items[0].Content.(map[string]any)
	if s, ok := c["service-id"].(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("cannot find 'service-id' for job '%s'", jobID)
}

func readJob(ctxt context.Context, jobID string) (*sdk.JobReadResponseBody, a.Payload, map[string]any, map[string]any, *jobOutContent, error) {
	serviceId, err := lookupServiceIDForJob(ctxt, jobID)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return readJobWithServiceID(ctxt, serviceId, jobID)
}

// readJobWithServiceID fetches job details when the serviceID is already known,
// bypassing the lookupServiceIDForJob aspects round-trip.
func readJobWithServiceID(ctxt context.Context, serviceId, jobID string) (*sdk.JobReadResponseBody, a.Payload, map[string]any, map[string]any, *jobOutContent, error) {
	req := &sdk.ReadServiceJobRequest{ServiceId: serviceId, JobId: jobID}
	job, pyld, err := sdk.ReadServiceJob(ctxt, req, CreateAdapter(true), logger)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// Try to fetch job.result.1 aspect (execution phase information)
	jobResultAspect := readJobResultAspect(ctxt, jobID)

	// Detect Nextflow jobs from result-content.$schema — no extra API call needed.
	// The server always inlines result-content in the job status response.
	var nextflowResultAspect map[string]any
	if contentMap, ok := job.ResultContent.(map[string]any); ok {
		if schema, ok := contentMap["$schema"].(string); ok && strings.Contains(schema, "nextflow") {
			// result-content IS the nextflow result object; use it directly.
			nextflowResultAspect = contentMap
		}
	}

	// Check the top-level result-content-urn / result-content-type fields.
	// Only set outContent when this is NOT a nextflow job (nextflow uses the
	// nextflowResultAspect display path instead).
	var outContent *jobOutContent
	if nextflowResultAspect == nil && job.ResultContentUrn != nil && *job.ResultContentUrn != "" {
		outContent = &jobOutContent{urn: *job.ResultContentUrn}
		// Treat application/json AND any application/vnd.ivcap.* type as JSON
		if job.ResultContentType != nil && isJSONContentType(*job.ResultContentType) && job.ResultContent != nil {
			jsonBytes, err := json.MarshalIndent(job.ResultContent, "", "  ")
			if err == nil {
				lines := strings.Split(string(jsonBytes), "\n")
				if len(lines) > jobResultPreviewLines {
					outContent.preview = strings.Join(lines[:jobResultPreviewLines], "\n") + "\n..."
				} else {
					outContent.preview = strings.Join(lines, "\n")
				}
			}
		}
	}
	// Also try the job.result.1 aspect for out-content-type / out-content-urn (fallback).
	if outContent == nil && nextflowResultAspect == nil && jobResultAspect != nil {
		outContent = fetchJobOutContent(ctxt, jobResultAspect)
	}

	return job, pyld, jobResultAspect, nextflowResultAspect, outContent, nil
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

// isJSONContentType returns true for application/json and IVCAP vendor types
// (application/vnd.ivcap.*), which are all JSON-based.
func isJSONContentType(ct string) bool {
	return ct == "application/json" || strings.HasPrefix(ct, "application/vnd.ivcap.")
}

func printJobListTable(list *aspect.ListResponseBody) {
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

func printJob(job *sdk.JobReadResponseBody, jobResultAspect map[string]any, nextflowResultAspect map[string]any, outContent *jobOutContent, wide bool) {

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
	} else if outContent != nil {
		// Job result aspect contains out-content-urn: display it with history token
		urnDisplay := fmt.Sprintf("%s (%s)", outContent.urn, MakeHistory(&outContent.urn))
		rows = append(rows, table.Row{"Result", urnDisplay})
		if outContent.preview != "" {
			rows = append(rows, table.Row{"", outContent.preview})
		}
	} else {
		// Fall back to old format: extract results_artifact_urn from result-content
		resultDisplay := "-"
		if job.ResultContent != nil {
			// Try to parse ResultContent as JSON and extract results_artifact_urn
			if contentMap, ok := job.ResultContent.(map[string]interface{}); ok {
				if artifactUrn, ok := contentMap["results_artifact_urn"].(string); ok && artifactUrn != "" {
					resultDisplay = fmt.Sprintf("%s (%s)", artifactUrn, MakeHistory(&artifactUrn))
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
