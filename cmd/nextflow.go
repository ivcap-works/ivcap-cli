// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	nf "github.com/ivcap-works/ivcap-cli/pkg/nextflow"
	api "github.com/ivcap-works/ivcap-core-api/http/aspect"
)

func init() {
	rootCmd.AddCommand(nextflowCmd)

	nextflowCmd.AddCommand(nextflowListCmd)
	addListFlags(nextflowListCmd)
	nextflowListCmd.Flags().StringVarP(&nextflowJobsJsonFilter, "content-path", "c", "", "json path filter on job's content ('$.images[*] ? (@.size > 10000)')")

	nextflowCmd.AddCommand(nextflowListPipelinesCmd)
	addListFlags(nextflowListPipelinesCmd)
	nextflowListPipelinesCmd.Flags().StringVarP(&nextflowPipelinesJsonFilter, "content-path", "c", "", "json path filter on pipeline's content ('$.name~=\".*rna.*\"')")

	nextflowCmd.AddCommand(nextflowGetJobCmd)
	nextflowCmd.AddCommand(nextflowJobResultCmd)
	nextflowJobResultCmd.Flags().StringVarP(&nextflowResultFile, "file", "f", "", "Optional: directory to download and extract result files into")
	nextflowJobResultCmd.Flags().BoolVar(&nextflowResultLogs, "logs", false, "Show the job logs")
	nextflowJobResultCmd.Flags().BoolVar(&nextflowResultOutput, "output", false, "Show the output directory contents")
	nextflowJobResultCmd.Flags().StringVar(&nextflowResultProcess, "process", "", "Show contents of a specific process (e.g., fastqc)")
	nextflowCmd.AddCommand(nextflowJobReportCmd)
	nextflowJobReportCmd.Flags().BoolVar(&nextflowReportMultiQC, "multiqc", false, "Serve the MultiQC report if available")
	nextflowCmd.AddCommand(nextflowCreateCmd)
	nextflowCmd.AddCommand(nextflowUpdateCmd)
	nextflowCmd.AddCommand(nextflowRetractCmd)
	nextflowCmd.AddCommand(nextflowRunCmd)
	nextflowCmd.AddCommand(nextflowEventsCmd)

	// events command flags
	nextflowEventsCmd.Flags().IntVar(&nextflowEventsMaxMessages, "max-messages", 0, "Maximum number of messages to return (0 = unlimited)")
	nextflowEventsCmd.Flags().IntVar(&nextflowEventsMaxWaitTime, "max-wait-time", 30, "Max wait time for new events in seconds")
	nextflowEventsCmd.Flags().StringVar(&nextflowEventsLastEventID, "last-event-id", "", "Last event ID to resume from")

	addFileFlag(nextflowCreateCmd, "Path to local tar/tgz containing ivcap.yaml or ivcap-tool.yaml with service-id field")
	nextflowCreateCmd.Flags().StringVar(&nextflowCreateFormat, "format", "", "Output format for nextflow create result [json, yaml]")

	addFileFlag(nextflowUpdateCmd, "Path to local tar/tgz containing ivcap.yaml or ivcap-tool.yaml")
	nextflowUpdateCmd.Flags().StringVar(&nextflowCreateFormat, "format", "", "Output format for nextflow update result [json, yaml]")

	// run is an alias for `ivcap job create`
	addFileFlag(nextflowRunCmd, "Path to job input file (use '-' for stdin)")
	addInputFormatFlag(nextflowRunCmd)
	nextflowRunCmd.Flags().StringVarP(&nextflowRunAspectURN, "aspect", "a", "", "URN of aspect containing job parameters")
	nextflowRunCmd.Flags().StringVarP(&nextflowRunSamplesheet, "samplesheet", "s", "", "Path to CSV samplesheet file (use '-' for stdin)")
	nextflowRunCmd.Flags().BoolVar(&nextflowRunWatchFlag, "watch", false, "if set, watch the job until it is finished")
	nextflowRunCmd.Flags().BoolVar(&nextflowRunStreamFlag, "stream", false, "if set, print job related events to stdout")
}

var nextflowCreateFormat string
var nextflowRunAspectURN string
var nextflowRunWatchFlag bool
var nextflowRunStreamFlag bool
var nextflowRunSamplesheet string
var nextflowJobsJsonFilter string
var nextflowPipelinesJsonFilter string
var nextflowResultFile string
var nextflowResultLogs bool
var nextflowResultOutput bool
var nextflowResultProcess string
var nextflowReportMultiQC bool
var nextflowEventsMaxMessages int
var nextflowEventsMaxWaitTime int
var nextflowEventsLastEventID string

var (
	nextflowCmd = &cobra.Command{
		Use:   "nextflow",
		Short: "Commands for working with Nextflow-based services",
	}

	nextflowListCmd = &cobra.Command{
		Use:     "list-jobs",
		Aliases: []string{"list-j", "jlist", "list-job", "list-runs", "job-list", "jobs-list"},
		Short:   "List recent Nextflow jobs",
		Long:    "List jobs that were created with Nextflow services (jobs with nextflow request schema)",
		RunE: func(cmd *cobra.Command, args []string) error {
			lr := createListRequest()
			if lr.OrderBy == nil {
				rb := "requested-at"
				lr.OrderBy = &rb
			}

			// Build JSON path filter to find jobs with nextflow.request.1 schema in content
			nfFilter := `$.["in-content"]["$schema"] == "urn:ivcap:schema:nextflow.request.1"`
			if nextflowJobsJsonFilter != "" {
				// Combine filters if user provided additional filter
				nfFilter = fmt.Sprintf("(%s) && (%s)", nfFilter, nextflowJobsJsonFilter)
			}

			selector := sdk.AspectSelector{
				SchemaPrefix:   JOB_SCHEMA,
				ListRequest:    *lr,
				IncludeContent: true,
				JsonFilter:     &nfFilter,
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

	nextflowListPipelinesCmd = &cobra.Command{
		Use:     "list-pipelines",
		Aliases: []string{"list-p", "plist", "list-pipeline", "list-services"},
		Short:   "List Nextflow pipeline services",
		Long:    "List service definitions for Nextflow pipelines",
		RunE: func(cmd *cobra.Command, args []string) error {
			lr := createListRequest()

			// Use field-based filter to find services with nextflow controller schema
			if lr.Filter == nil {
				filterStr := "controller-schema~=urn:ivcap:schema.service.nextflow.1"
				lr.Filter = &filterStr
			} else {
				// Combine with existing filter
				combinedFilter := fmt.Sprintf("(%s) && (controller-schema~=urn:ivcap:schema.service.nextflow.1)", *lr.Filter)
				lr.Filter = &combinedFilter
			}

			selector := sdk.AspectSelector{
				SchemaPrefix:   "urn:ivcap:schema.service.2",
				ListRequest:    *lr,
				IncludeContent: true,
			}

			ctxt := context.Background()
			if list, res, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					printNextflowPipelinesTable(list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	nextflowCreateCmd = &cobra.Command{
		Use:   "create [flags] -f package.tar|package.tgz",
		Short: "Create a Nextflow service definition from a local archive",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Service ID is extracted from the archive's ivcap.yaml
			return runNextflowCreateOrUpdate(context.Background(), "")
		},
	}

	nextflowUpdateCmd = &cobra.Command{
		Use:   "update [service-id] [flags] -f package.tar|package.tgz",
		Short: "Update a Nextflow service definition from a local archive",
		Long:  "Update a Nextflow service definition from a local archive. Service ID is extracted from the archive's ivcap.yaml or ivcap-tool.yaml if not provided as an argument.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceID := ""
			if len(args) > 0 {
				serviceID = GetHistory(args[0])
			}
			return runNextflowCreateOrUpdate(context.Background(), serviceID)
		},
	}

	nextflowGetJobCmd = &cobra.Command{
		Use:     "job-get [flags] job_id",
		Aliases: []string{"get-job", "get", "get-run"},
		Short:   "Get status or results of a Nextflow job",
		Long:    "Fetch details about a single Nextflow job without needing the service/pipeline URN",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := GetHistory(args[0])
			ctxt := context.Background()
			return readDisplayJob(ctxt, jobID)
		},
	}

	nextflowJobResultCmd = &cobra.Command{
		Use:     "job-result [flags] job_id [-f directory]",
		Aliases: []string{"result", "results"},
		Short:   "List or download files from a Nextflow job result artifact",
		Long:    "Download and access the result artifact from a Nextflow job. Without flags, shows summary. With -f, downloads/extracts files into the specified directory.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := GetHistory(args[0])
			ctxt := context.Background()
			return handleNextflowJobResult(ctxt, jobID, nextflowResultFile)
		},
	}

	nextflowJobReportCmd = &cobra.Command{
		Use:     "job-view job_id [--multiqc]",
		Aliases: []string{"job-report", "report"},
		Short:   "View Nextflow job execution report in a web browser",
		Long:    "Download the output directory from a Nextflow job and serve the index.html file in a local web server. The report will be automatically opened in your browser if possible. With --multiqc, serves the MultiQC report if available.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := GetHistory(args[0])
			ctxt := context.Background()
			return handleNextflowJobReport(ctxt, jobID)
		},
	}

	nextflowRetractCmd = &cobra.Command{
		Use:   "retract service-id [flags]",
		Short: "Retract the service aspect(s) created by 'nextflow create'",
		Long:  "Query and retract the service description aspect(s) for a given service ID. This is the opposite of 'nextflow create'.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceID := GetHistory(args[0])
			ctxt := context.Background()
			return retractNextflowService(ctxt, serviceID)
		},
	}

	nextflowRunCmd = &cobra.Command{
		Use:   "run [flags] service-id [-f job-input|-] [-a aspect-urn] [-s|--samplesheet file.csv|-] [--watch] [--stream]",
		Short: "Alias for 'ivcap job create'",
		Long:  "Alias for 'ivcap job create' (creates a job for a given service ID with provided input).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			serviceID := GetHistory(args[0])

			// Validate that at least one input source is provided
			if fileName == "" && nextflowRunAspectURN == "" && nextflowRunSamplesheet == "" {
				cobra.CheckErr("Missing input: provide '-f job-file|-', '-a aspectURN', or '--samplesheet file.csv|-'")
			}

			// Validate that stdin is not used by both file and samplesheet
			if fileName == "-" && nextflowRunSamplesheet == "-" {
				cobra.CheckErr("Cannot use stdin ('-') for both --file and --samplesheet simultaneously")
			}

			var pyld a.Payload
			if fileName != "" {
				if pyld, err = payloadFromFile(fileName, inputFormat); err != nil {
					cobra.CheckErr(fmt.Sprintf("While reading job file '%s' - %v", fileName, err))
				}
			}
			if nextflowRunAspectURN != "" {
				j := fmt.Sprintf(CREATE_FROM_ASPECT, nextflowRunAspectURN, serviceID)
				if pyld, err = a.LoadPayloadFromBytes([]byte(j), false); err != nil {
					cobra.CheckErr(fmt.Sprintf("While reading job aspect '%s' - %v", nextflowRunAspectURN, err))
				}
			}

			// Process samplesheet if provided
			if nextflowRunSamplesheet != "" {
				samples, err := parseSamplesheet(nextflowRunSamplesheet)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("While reading samplesheet '%s' - %v", nextflowRunSamplesheet, err))
				}

				// Merge samples into the payload
				if pyld, err = mergeSamplesIntoPayload(pyld, samples); err != nil {
					cobra.CheckErr(fmt.Sprintf("While merging samples into payload - %v", err))
				}
			}

			// ╔══════════════════════════════════════════════════════════════════════════╗
			// ║ TEMPORARY WORKAROUND - REMOVE WHEN IVCAP API NO LONGER REQUIRES $schema ║
			// ╚══════════════════════════════════════════════════════════════════════════╝
			// Ensure the payload has a $schema field (required by IVCAP API for now)
			pyld, err = a.EnsureSchemaField(pyld)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("While ensuring $schema field - %v", err))
			}

			res, jobCreate, err := sdk.CreateServiceJobRaw(ctxt, serviceID, pyld, 0, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			if jobCreate != nil {
				// Use local flags for behaviour.
				watchFlag = nextflowRunWatchFlag
				streamFlag = nextflowRunStreamFlag
				return waitForResult(ctxt, jobCreate)
			}
			reply, err := res.AsObject()
			if err != nil {
				return err
			}
			jobID, ok := reply["job-id"].(string)
			if !ok {
				cobra.CheckErr("Cannot find job ID in response")
			}
			return readDisplayJob(ctxt, jobID)
		},
	}

	nextflowEventsCmd = &cobra.Command{
		Use:   "events [flags] service-id job-id",
		Short: "Stream events for a Nextflow job",
		Long: `Stream job-related events in real-time for a Nextflow service. Events are displayed as they occur.

Examples:
  ivcap nextflow events urn:ivcap:service:123 urn:ivcap:job:456
  ivcap nextflow events --max-messages 10 service-id job-id
  ivcap nextflow events --last-event-id abc123 service-id job-id`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceID := GetHistory(args[0])
			jobID := GetHistory(args[1])
			ctxt := context.Background()

			var lastID *string
			if nextflowEventsLastEventID != "" {
				lastID = &nextflowEventsLastEventID
			}
			return streamJobEvents(ctxt, serviceID, jobID, lastID, nextflowEventsMaxMessages)
		},
	}
)

func runNextflowCreateOrUpdate(ctxt context.Context, serviceID string) error {
	if fileName == "" {
		cobra.CheckErr("Missing archive file '-f package.tar|package.tgz'")
	}
	if fileName == "-" {
		cobra.CheckErr("Archive file must be a local path; stdin ('-') is not supported")
	}

	tool, _, err := nf.LoadToolHeaderFromArchivePath(fileName)
	if err != nil {
		return err
	}
	if tool == nil {
		return fmt.Errorf("neither %q nor %q found in archive %q", nf.SimpleToolFileName, nf.ToolFileName, fileName)
	}

	// Resolve service ID from provided ID or tool header
	resolvedServiceID, err := nf.ResolveServiceID(serviceID, tool)
	if err != nil {
		return err
	}

	adapter := CreateAdapter(true)
	artifactID, err := nf.UploadArchiveAsArtifact(ctxt, tool.Name, fileName, DEF_CHUNK_SIZE, adapter, silent, logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("while uploading archive as artifact: %v", err))
	}

	svc := nf.BuildServiceDescription(tool, resolvedServiceID, artifactID)
	aspectID, err := nf.UpsertServiceDescriptionAspect(ctxt, resolvedServiceID, svc, adapter, logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("while publishing service description aspect: %v", err))
	}

	res := &nf.CreateOutput{
		OK:                    true,
		ServiceID:             resolvedServiceID,
		PipelineArtifactURN:   artifactID,
		ServiceAspectRecordID: aspectID,
		ServiceDescription:    svc,
	}
	return printNextflowCreateOutput(res)
}

func printNextflowCreateOutput(out *nf.CreateOutput) error {
	// Default output is human readable. `--format json|yaml` emits machine readable.
	switch nextflowCreateFormat {
	case "":
		if _, err := fmt.Fprintln(os.Stdout, "Nextflow service created successfully"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "  service:  %s\n", out.ServiceID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "  pipeline: %s\n", out.PipelineArtifactURN); err != nil {
			return err
		}
		if out.ServiceAspectRecordID != "" {
			if _, err := fmt.Fprintf(os.Stdout, "  aspect:   %s\n", out.ServiceAspectRecordID); err != nil {
				return err
			}
		}
		return nil
	case "json":
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(os.Stdout, string(b)); err != nil {
			return err
		}
		return nil
	case "yaml":
		b, err := yaml.Marshal(out)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(os.Stdout, string(b)); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported --format %q (expected json|yaml)", nextflowCreateFormat)
	}
}

// parseSamplesheet reads a CSV file and converts it to a slice of maps.
// The first line is treated as column headers, and subsequent lines become sample records.
func parseSamplesheet(fileName string) ([]map[string]interface{}, error) {
	var reader *os.File
	var err error

	if fileName == "-" {
		reader = os.Stdin
	} else {
		// #nosec G304 - fileName is provided by user via CLI flag for intentional file access
		reader, err = os.Open(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to open samplesheet: %w", err)
		}
		defer func() { _ = reader.Close() }()
	}

	// Create CSV reader
	csvReader := csv.NewReader(reader)

	// Read header row
	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	if len(headers) == 0 {
		return nil, fmt.Errorf("samplesheet must have at least one column")
	}

	// Read all records
	var samples []map[string]interface{}
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		// Convert record to map
		sample := make(map[string]interface{})
		for i, value := range record {
			if i < len(headers) {
				sample[headers[i]] = value
			}
		}
		samples = append(samples, sample)
	}

	return samples, nil
}

// mergeSamplesIntoPayload merges the samples array into the payload under the 'samples' key.
// If the payload is nil, creates a new one.
func mergeSamplesIntoPayload(pyld a.Payload, samples []map[string]interface{}) (a.Payload, error) {
	var payloadMap map[string]interface{}

	if pyld != nil {
		// Convert existing payload to map
		obj, err := pyld.AsObject()
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to object: %w", err)
		}
		payloadMap = obj
	} else {
		// Create new payload map
		payloadMap = make(map[string]interface{})
	}

	// Add samples to the payload
	payloadMap["samples"] = samples

	// Convert back to JSON and create new payload
	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload with samples: %w", err)
	}

	newPayload, err := a.LoadPayloadFromBytes(jsonBytes, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload from merged data: %w", err)
	}

	return newPayload, nil
}

// printNextflowPipelinesTable prints a table of Nextflow pipeline services
func printNextflowPipelinesTable(list *api.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"ID", "Name", "Description"})
	tw2.SetStyle(table.StyleLight)
	tw2.Style().Options.SeparateRows = true // Add row separators
	rows := make([]table.Row, len(list.Items))
	for i, p := range list.Items {
		c := p.Content.(map[string]any)

		// Extract entity (service ID)
		entity := safeString(p.Entity)

		// Extract name
		name := "???"
		if n, ok := c["name"].(string); ok {
			name = n
		}

		// Extract and truncate description to 3 lines, removing empty lines
		description := ""
		if d, ok := c["description"].(string); ok {
			// Remove empty lines before truncating
			cleanDesc := removeEmptyLines(d)
			description = truncateToLines(cleanDesc, 3)
		}

		rows[i] = table.Row{MakeHistory(&entity), name, description}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 3, WidthMax: 60, WidthMaxEnforcer: WrapSoftSoft},
	})

	p := []table.Row{}
	if list.AtTime != nil {
		p = append(p, table.Row{"At Time", safeDate(list.AtTime, false)})
	}
	p = append(p, table.Row{"Pipelines", tw2.Render()})
	p = addNextPageRow(findNextAspectPage(list.Links), p)
	tw.AppendRows(p)

	fmt.Printf("\n%s\n\n", tw.Render())
}

// removeEmptyLines removes empty or whitespace-only lines from text
func removeEmptyLines(text string) string {
	if text == "" {
		return ""
	}

	lines := []string{}
	currentLine := ""

	for _, ch := range text {
		if ch == '\n' {
			trimmed := strings.TrimSpace(currentLine)
			if trimmed != "" {
				lines = append(lines, trimmed)
			}
			currentLine = ""
		} else {
			currentLine += string(ch)
		}
	}

	// Add the last line if not empty
	trimmed := strings.TrimSpace(currentLine)
	if trimmed != "" {
		lines = append(lines, trimmed)
	}

	return strings.Join(lines, "\n")
}

// truncateToLines truncates text to a maximum number of lines, adding ellipsis if truncated
func truncateToLines(text string, maxLines int) string {
	if text == "" {
		return ""
	}

	lines := []string{}
	currentLine := ""

	for _, ch := range text {
		if ch == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
			if len(lines) >= maxLines {
				break
			}
		} else {
			currentLine += string(ch)
		}
	}

	// Add the last line if we haven't reached maxLines yet
	if currentLine != "" && len(lines) < maxLines {
		lines = append(lines, currentLine)
	}

	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}

	// Check if there's more content after maxLines
	remainingText := text
	for i := 0; i < len(lines); i++ {
		var idx int
		if i == 0 {
			idx = len(lines[i])
		} else {
			idx = len(lines[i]) + 1 // +1 for the newline
		}
		if idx < len(remainingText) {
			remainingText = remainingText[idx:]
		} else {
			remainingText = ""
		}
	}

	if remainingText != "" && len(remainingText) > 0 {
		result += "..."
	}

	return result
}

// handleNextflowJobResult handles listing or extracting files from a Nextflow job result artifact
func handleNextflowJobResult(ctxt context.Context, jobID string, filePath string) error {
	adapter := CreateAdapter(true)

	// Get the job to extract result artifact URN
	job, _, _, _, err := readJob(ctxt, jobID)
	if err != nil {
		return fmt.Errorf("failed to read job: %w", err)
	}

	// Try to fetch the nextflow result aspect (new format) first
	nextflowResultContent, err := readNextflowResultAspect(ctxt, jobID)
	if err == nil && nextflowResultContent != nil {
		// New format found, use it
		return handleNextflowJobResultNewFormatWithContent(ctxt, jobID, nextflowResultContent, adapter)
	}

	// Check if this is the new format with separate log_urn, output_urn, and results
	if nextflowResultLogs || nextflowResultOutput || nextflowResultProcess != "" {
		return handleNextflowJobResultNewFormat(ctxt, jobID, job, adapter)
	}

	// Extract results_artifact_urn from result-content (old format)
	var artifactURN string
	if job.ResultContent != nil {
		if contentMap, ok := job.ResultContent.(map[string]interface{}); ok {
			if urn, ok := contentMap["results_artifact_urn"].(string); ok && urn != "" {
				artifactURN = urn
			}
		}
	}

	if artifactURN == "" {
		return fmt.Errorf("job '%s' has no result artifact (job may still be running or failed)", jobID)
	}

	// Try to load from nextflow cache first
	var data []byte
	cachedData, found := loadNextflowResultCache()
	cachedArtifactID := getNextflowCachedArtifactID()

	if found && cachedArtifactID == artifactURN {
		logger.Debug("using cached nextflow result artifact", log.String("id", artifactURN))
		data = cachedData
	} else {
		// Download the artifact
		var mimeType string
		data, mimeType, err = downloadTarArtifact(ctxt, artifactURN, adapter)
		if err != nil {
			return err
		}

		// Verify it's a tar file
		if !isTarFile(mimeType, data) {
			return fmt.Errorf("result artifact '%s' is not a tar or tar.gz file (mime-type: %s)", artifactURN, mimeType)
		}

		// Always cache nextflow results (replaces any previous cached nextflow result)
		if err := saveNextflowResultCache(artifactURN, data); err != nil {
			logger.Warn("failed to cache nextflow result artifact", log.Error(err))
		}
	}

	// If no file specified, list the contents
	if filePath == "" {
		files, err := listTarFiles(data)
		if err != nil {
			return fmt.Errorf("failed to list tar contents: %w", err)
		}
		printNextflowResultTable(jobID, files)
		return nil
	}

	// Resolve history tag if provided (e.g., @2 -> actual filename)
	resolvedFilePath := GetHistory(filePath)

	// Extract and display the specified file
	fileData, err := extractFileFromTar(data, resolvedFilePath)
	if err != nil {
		return fmt.Errorf("failed to extract file '%s': %w", filePath, err)
	}

	// Write to stdout
	_, err = os.Stdout.Write(fileData)
	return err
}

// handleNextflowJobResultNewFormat handles the new nextflow result format with separate artifacts
func handleNextflowJobResultNewFormat(ctxt context.Context, jobID string, job *sdk.JobReadResponseBody, adapter *a.Adapter) error {
	// Extract the new format fields from result-content
	if job.ResultContent == nil {
		return fmt.Errorf("job '%s' has no result content", jobID)
	}

	contentMap, ok := job.ResultContent.(map[string]interface{})
	if !ok {
		return fmt.Errorf("job '%s' result content is not a map", jobID)
	}

	// Handle --logs flag
	if nextflowResultLogs {
		logURN, ok := contentMap["log_urn"].(string)
		if !ok || logURN == "" {
			return fmt.Errorf("job '%s' has no log artifact", jobID)
		}
		return displayArtifactContent(ctxt, logURN, adapter, "Job Logs")
	}

	// Handle --output flag
	if nextflowResultOutput {
		outputURN, ok := contentMap["output_urn"].(string)
		if !ok || outputURN == "" {
			return fmt.Errorf("job '%s' has no output artifact", jobID)
		}
		return displayArtifactFiles(ctxt, outputURN, adapter, "Output Directory")
	}

	// Handle --process flag
	if nextflowResultProcess != "" {
		results, ok := contentMap["results"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("job '%s' has no results in new format", jobID)
		}
		processURN, ok := results[nextflowResultProcess].(string)
		if !ok || processURN == "" {
			return fmt.Errorf("process '%s' not found in results", nextflowResultProcess)
		}
		return displayArtifactFiles(ctxt, processURN, adapter, fmt.Sprintf("Process: %s", nextflowResultProcess))
	}

	// No specific flag, show summary
	return displayNextflowResultSummary(ctxt, jobID, contentMap, adapter)
}

// displayArtifactContent downloads and displays the raw content of an artifact (for logs)
func displayArtifactContent(ctxt context.Context, artifactURN string, adapter *a.Adapter, title string) error {
	data, mimeType, err := downloadTarArtifact(ctxt, artifactURN, adapter)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}

	// For logs, just output the raw content
	if strings.Contains(mimeType, "text") || strings.Contains(mimeType, "plain") {
		_, err := os.Stdout.Write(data)
		return err
	}

	// If it's gzipped or tar, try to extract and display
	if isTarFile(mimeType, data) {
		// Try to list contents using MANIFEST.csv
		manifest, err := extractManifest(data)
		if err == nil && len(manifest) > 0 {
			printManifestTable(title, manifest)
			return nil
		}
	}

	// Fall back to raw content
	_, err = os.Stdout.Write(data)
	return err
}

// displayArtifactFiles downloads an artifact and displays its file listing
func displayArtifactFiles(ctxt context.Context, artifactURN string, adapter *a.Adapter, title string) error {
	data, mimeType, err := downloadTarArtifact(ctxt, artifactURN, adapter)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}

	// Verify it's a tar file
	if !isTarFile(mimeType, data) {
		return fmt.Errorf("artifact is not a tar or tar.gz file (mime-type: %s)", mimeType)
	}

	// Try to extract and display MANIFEST.csv
	manifest, err := extractManifest(data)
	if err != nil {
		return fmt.Errorf("failed to read manifest from artifact: %w", err)
	}

	if len(manifest) == 0 {
		return fmt.Errorf("no files found in artifact manifest")
	}

	printManifestTable(title, manifest)
	return nil
}

// displayNextflowResultSummary displays a summary of the new format results
func displayNextflowResultSummary(ctxt context.Context, jobID string, contentMap map[string]interface{}, adapter *a.Adapter) error {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false

	rows := []table.Row{}

	// Status
	if status, ok := contentMap["status"].(string); ok {
		rows = append(rows, table.Row{"Status", status})
	}

	// Log artifact
	if logURN, ok := contentMap["log_urn"].(string); ok && logURN != "" {
		rows = append(rows, table.Row{"Log", MakeHistory(&logURN)})
	}

	// Output artifact
	if outputURN, ok := contentMap["output_urn"].(string); ok && outputURN != "" {
		rows = append(rows, table.Row{"Output", MakeHistory(&outputURN)})
	}

	// Results (processes)
	if results, ok := contentMap["results"].(map[string]interface{}); ok && len(results) > 0 {
		rows = append(rows, table.Row{"", ""})
		rows = append(rows, table.Row{"Processes", ""})
		for procName, procURN := range results {
			if urn, ok := procURN.(string); ok && urn != "" {
				rows = append(rows, table.Row{"  " + procName, MakeHistory(&urn)})
			}
		}
	}

	tw.AppendRows(rows)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})

	fmt.Printf("\n%s\n\n", tw.Render())
	fmt.Printf("Use --logs, --output, or --process=NAME to view details\n\n")

	return nil
}

// ManifestEntry represents a row from MANIFEST.csv
type ManifestEntry struct {
	Name     string
	Size     int64
	MimeType string
	Path     string
}

// extractManifest extracts MANIFEST.csv from a tar artifact and parses it
func extractManifest(data []byte) ([]ManifestEntry, error) {
	manifestData, err := extractFileFromTar(data, "MANIFEST.csv")
	if err != nil {
		return nil, fmt.Errorf("MANIFEST.csv not found in artifact: %w", err)
	}

	// Parse CSV
	reader := csv.NewReader(strings.NewReader(string(manifestData)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse MANIFEST.csv: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("MANIFEST.csv has no data rows")
	}

	var entries []ManifestEntry
	// Skip header row
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue
		}

		size, err := strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			size = 0
		}

		entries = append(entries, ManifestEntry{
			Name:     record[0],
			Size:     size,
			MimeType: record[2],
			Path:     record[3],
		})
	}

	return entries, nil
}

// printManifestTable prints a table of manifest entries
func printManifestTable(title string, entries []ManifestEntry) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"Name", "Size", "Type"})
	tw2.SetStyle(table.StyleLight)

	rows := make([]table.Row, len(entries))
	for i, e := range entries {
		rows[i] = table.Row{e.Path, safeBytes(&e.Size), e.MimeType}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
	})

	p := []table.Row{
		{title, ""},
		{"", tw2.Render()},
	}
	tw.AppendRows(p)

	fmt.Printf("\n%s\n\n", tw.Render())
}

// printNextflowResultTable prints a table of Nextflow result files with history tags
func printNextflowResultTable(jobID string, files []tarFileInfo) {
	// Register job ID first to ensure it gets @1
	jobHistory := MakeHistory(&jobID)

	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"File", "Size", "Mode"})
	tw2.SetStyle(table.StyleLight)

	rows := make([]table.Row, len(files))
	for i, f := range files {
		// Create history tag for the file name and combine with the actual filename
		fileName := f.Name
		fileHistory := MakeHistory(&fileName)
		// Show both history tag and filename (e.g., "@2 results.tar.gz")
		fileDisplay := fmt.Sprintf("%s %s", fileHistory, fileName)
		rows[i] = table.Row{fileDisplay, safeBytes(&f.Size), f.Mode}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
	})

	p := []table.Row{
		{"Job ID", jobHistory},
		{"", ""},
		{"Files", tw2.Render()},
	}
	tw.AppendRows(p)

	fmt.Printf("\n%s\n\n", tw.Render())
}

// getNextflowCachePath returns the path to the nextflow result cache file
func getNextflowCachePath() (string, error) {
	cacheDir, err := getTarCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "nextflow-result.cache"), nil
}

// getNextflowCacheMetaPath returns the path to the nextflow cache metadata file
func getNextflowCacheMetaPath() (string, error) {
	cacheDir, err := getTarCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "nextflow-result.meta"), nil
}

// saveNextflowResultCache saves a nextflow result artifact to the cache (replacing any existing one)
func saveNextflowResultCache(artifactID string, data []byte) error {
	cachePath, err := getNextflowCachePath()
	if err != nil {
		return err
	}

	metaPath, err := getNextflowCacheMetaPath()
	if err != nil {
		return err
	}

	cacheDir, err := getTarCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil { // #nosec G301 -- cache directory under user's home
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write the artifact data
	if err := os.WriteFile(cachePath, data, 0644); err != nil { // #nosec G306 -- cache file under user's home
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Write metadata (artifact ID)
	if err := os.WriteFile(metaPath, []byte(artifactID), 0644); err != nil { // #nosec G306 -- metadata file under user's home
		return fmt.Errorf("failed to write cache metadata: %w", err)
	}

	logger.Debug("cached nextflow result artifact", log.String("id", artifactID), log.String("path", cachePath))
	return nil
}

// loadNextflowResultCache loads the cached nextflow result artifact
func loadNextflowResultCache() ([]byte, bool) {
	cachePath, err := getNextflowCachePath()
	if err != nil {
		return nil, false
	}

	data, err := os.ReadFile(cachePath) // #nosec G304 -- cachePath is constructed from safe getNextflowCachePath
	if err != nil {
		return nil, false
	}

	return data, true
}

// getNextflowCachedArtifactID returns the artifact ID of the cached nextflow result
func getNextflowCachedArtifactID() string {
	metaPath, err := getNextflowCacheMetaPath()
	if err != nil {
		return ""
	}

	data, err := os.ReadFile(metaPath) // #nosec G304 -- metaPath is constructed from safe getNextflowCacheMetaPath
	if err != nil {
		return ""
	}

	return string(data)
}

// readNextflowResultAspect reads the nextflow.result.1 aspect for a job
func readNextflowResultAspect(ctxt context.Context, jobID string) (map[string]interface{}, error) {
	selector := sdk.AspectSelector{
		Entity:         jobID,
		SchemaPrefix:   "urn:ivcap:schema:nextflow.result.1",
		IncludeContent: true,
	}

	list, _, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger)
	if err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no nextflow result aspect found")
	}

	// Get the first (and should be only) item
	if content, ok := list.Items[0].Content.(map[string]interface{}); ok {
		return content, nil
	}

	return nil, fmt.Errorf("nextflow result aspect content is not a map")
}

// handleNextflowJobResultNewFormatWithContent handles the new nextflow result format using the aspect content directly
func handleNextflowJobResultNewFormatWithContent(ctxt context.Context, jobID string, contentMap map[string]interface{}, adapter *a.Adapter) error {
	// Handle --logs flag
	if nextflowResultLogs {
		logURN, ok := contentMap["log_urn"].(string)
		if !ok || logURN == "" {
			return fmt.Errorf("job '%s' has no log artifact", jobID)
		}
		if nextflowResultFile != "" {
			return downloadArtifactAsLog(ctxt, logURN, adapter, jobID, nextflowResultFile)
		}
		return displayArtifactContent(ctxt, logURN, adapter, "Job Logs")
	}

	// Handle --output flag
	if nextflowResultOutput {
		outputURN, ok := contentMap["output_urn"].(string)
		if !ok || outputURN == "" {
			return fmt.Errorf("job '%s' has no output artifact", jobID)
		}
		if nextflowResultFile != "" {
			return downloadArtifactAsFiles(ctxt, outputURN, adapter, nextflowResultFile)
		}
		return displayArtifactFiles(ctxt, outputURN, adapter, "Output Directory")
	}

	// Handle --process flag
	if nextflowResultProcess != "" {
		results, ok := contentMap["results"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("job '%s' has no results in new format", jobID)
		}
		processURN, ok := results[nextflowResultProcess].(string)
		if !ok || processURN == "" {
			return fmt.Errorf("process '%s' not found in results", nextflowResultProcess)
		}
		if nextflowResultFile != "" {
			return downloadArtifactAsFiles(ctxt, processURN, adapter, nextflowResultFile)
		}
		return displayArtifactFiles(ctxt, processURN, adapter, fmt.Sprintf("Process: %s", nextflowResultProcess))
	}

	// If -f flag is provided without other flags, extract the summary artifacts into the directory
	if nextflowResultFile != "" {
		return downloadNextflowResultSummary(ctxt, jobID, contentMap, adapter, nextflowResultFile)
	}

	// No specific flag, show summary
	return displayNextflowResultSummary(ctxt, jobID, contentMap, adapter)
}

// downloadArtifactAsLog downloads a log artifact and saves it as log.{jobID}.txt in the directory
func downloadArtifactAsLog(ctxt context.Context, artifactURN string, adapter *a.Adapter, jobID string, dirPath string) error {
	data, _, err := downloadTarArtifact(ctxt, artifactURN, adapter)
	if err != nil {
		return fmt.Errorf("failed to download log artifact: %w", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil { // #nosec G301 -- user-specified directory
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Extract job UUID from job ID (format: urn:ivcap:job:UUID)
	jobUUID := jobID
	if parts := strings.Split(jobID, ":"); len(parts) > 0 {
		jobUUID = parts[len(parts)-1]
	}

	// Save log file
	logPath := filepath.Join(dirPath, fmt.Sprintf("log.%s.txt", jobUUID))
	if err := os.WriteFile(logPath, data, 0644); err != nil { // #nosec G306 -- user-specified directory
		return fmt.Errorf("failed to write log file: %w", err)
	}

	fmt.Printf("Saved log to: %s\n", logPath)
	return nil
}

// downloadArtifactAsFiles downloads a tar artifact and extracts all files to the directory
func downloadArtifactAsFiles(ctxt context.Context, artifactURN string, adapter *a.Adapter, dirPath string) error {
	data, mimeType, err := downloadTarArtifact(ctxt, artifactURN, adapter)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}

	// Verify it's a tar file
	if !isTarFile(mimeType, data) {
		return fmt.Errorf("artifact is not a tar or tar.gz file (mime-type: %s)", mimeType)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil { // #nosec G301 -- user-specified directory
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Extract tar contents
	tr, err := openTarReader(data)
	if err != nil {
		return fmt.Errorf("failed to open tar reader: %w", err)
	}

	count := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Skip directories
		if header.Typeflag == 53 { // tar.TypeDir
			continue
		}

		// Create file path
		filePath := filepath.Join(dirPath, filepath.FromSlash(header.Name))
		fileDir := filepath.Dir(filePath)

		// Create directory if needed
		if err := os.MkdirAll(fileDir, 0755); err != nil { // #nosec G301 -- user-specified directory
			return fmt.Errorf("failed to create directory %s: %w", fileDir, err)
		}

		// Extract file
		file, err := os.Create(filePath) // #nosec G304 -- user-specified directory
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(file, tr)
		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", filePath, err)
		}

		count++
	}

	fmt.Printf("Extracted %d files to: %s\n", count, dirPath)
	return nil
}

// downloadNextflowResultSummary downloads all artifacts from the summary and extracts them into subdirectories
func downloadNextflowResultSummary(ctxt context.Context, jobID string, contentMap map[string]interface{}, adapter *a.Adapter, basePath string) error {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil { // #nosec G301 -- user-specified directory
		return fmt.Errorf("failed to create directory %s: %w", basePath, err)
	}

	// Download logs if available
	if logURN, ok := contentMap["log_urn"].(string); ok && logURN != "" {
		if err := downloadArtifactAsLog(ctxt, logURN, adapter, jobID, basePath); err != nil {
			return fmt.Errorf("failed to download logs: %w", err)
		}
	}

	// Download output if available
	if outputURN, ok := contentMap["output_urn"].(string); ok && outputURN != "" {
		outputDir := filepath.Join(basePath, "output")
		if err := downloadArtifactAsFiles(ctxt, outputURN, adapter, outputDir); err != nil {
			return fmt.Errorf("failed to download output: %w", err)
		}
	}

	// Download results (processes) if available
	if results, ok := contentMap["results"].(map[string]interface{}); ok {
		for procName, procURN := range results {
			if urn, ok := procURN.(string); ok && urn != "" {
				procDir := filepath.Join(basePath, "results", procName)
				if err := downloadArtifactAsFiles(ctxt, urn, adapter, procDir); err != nil {
					return fmt.Errorf("failed to download results for process %s: %w", procName, err)
				}
			}
		}
	}

	return nil
}

// handleNextflowJobReport downloads the output directory and serves the execution report in a web browser
func handleNextflowJobReport(ctxt context.Context, jobID string) error {
	adapter := CreateAdapter(true)

	// Try to fetch the nextflow result aspect (new format) first
	nextflowResultContent, err := readNextflowResultAspect(ctxt, jobID)
	if err != nil {
		return fmt.Errorf("failed to read nextflow result aspect: %w", err)
	}

	if nextflowResultContent == nil {
		return fmt.Errorf("job '%s' has no result content", jobID)
	}

	// Handle --multiqc flag
	if nextflowReportMultiQC {
		results, ok := nextflowResultContent["results"].(map[string]interface{})
		if !ok || results == nil {
			return fmt.Errorf("job '%s' has no results in new format", jobID)
		}

		multiqcURN, ok := results["multiqc"].(string)
		if !ok || multiqcURN == "" {
			return fmt.Errorf("no multiqc process found in job results")
		}

		return serveMultiQCReport(ctxt, adapter, multiqcURN)
	}

	// Get the output URN
	outputURN, ok := nextflowResultContent["output_urn"].(string)
	if !ok || outputURN == "" {
		return fmt.Errorf("job '%s' has no output artifact", jobID)
	}

	// Create a temporary directory for the report
	tmpDir, err := os.MkdirTemp("", "nextflow-report-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Download and extract output files
	data, mimeType, err := downloadTarArtifact(ctxt, outputURN, adapter)
	if err != nil {
		return fmt.Errorf("failed to download output artifact: %w", err)
	}

	// Verify it's a tar file
	if !isTarFile(mimeType, data) {
		return fmt.Errorf("output artifact is not a tar or tar.gz file (mime-type: %s)", mimeType)
	}

	// Extract tar contents
	tr, err := openTarReader(data)
	if err != nil {
		return fmt.Errorf("failed to open tar reader: %w", err)
	}

	var reportFile string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Skip directories
		if header.Typeflag == 53 { // tar.TypeDir
			continue
		}

		// Create file path
		filePath := filepath.Join(tmpDir, filepath.FromSlash(header.Name))
		fileDir := filepath.Dir(filePath)

		// Create directory if needed
		if err := os.MkdirAll(fileDir, 0755); err != nil { // #nosec G301 -- temporary directory
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file
		file, err := os.Create(filePath) // #nosec G304 -- temporary directory
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(file, tr)
		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}

		// Check if this is the index.html file
		if strings.HasSuffix(header.Name, "index.html") {
			reportFile = filePath
		}
	}

	if reportFile == "" {
		return fmt.Errorf("no index.html file found in output directory")
	}

	// Start a web server on an available port
	listener, err := listenOnAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to start web server: %w", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://localhost:%d", port)

	// Serve the report and all extracted files
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Serve the index.html as root
			http.ServeFile(w, r, reportFile)
		} else {
			// Serve other files from the extracted tar contents
			// Remove leading slash and join with tmpDir
			requestPath := strings.TrimPrefix(r.URL.Path, "/")
			filePath := filepath.Join(tmpDir, filepath.FromSlash(requestPath))

			// Security check: ensure the requested file is within tmpDir
			absFilePath, err := filepath.Abs(filePath)
			absTmpDir, err2 := filepath.Abs(tmpDir)
			if err != nil || err2 != nil || !strings.HasPrefix(absFilePath, absTmpDir) {
				http.NotFound(w, r)
				return
			}

			// Try to serve the file
			if _, err := os.Stat(filePath); err == nil { // #nosec G703 -- path already validated above
				http.ServeFile(w, r, filePath)
				return
			}

			// If file not found at exact path, try to find it by searching tmpDir
			// This handles cases where files are in subdirectories
			foundPath := findFileByName(tmpDir, filepath.Base(requestPath))
			if foundPath != "" {
				http.ServeFile(w, r, foundPath)
				return
			}

			// Not found
			http.NotFound(w, r)
		}
	})

	// Print URL
	fmt.Printf("\n\nNextflow Execution Report\n")
	fmt.Printf("========================\n")
	fmt.Printf("Report URL: %s\n\n", url)

	// Try to open in browser
	openBrowser(url)

	// Start server (blocking)
	fmt.Printf("Press Ctrl+C to stop the server\n\n")
	if err := http.Serve(listener, nil); err != nil { // #nosec G114 -- local development server
		return fmt.Errorf("web server error: %w", err)
	}

	return nil
}

// listenOnAvailablePort finds and returns an available TCP port
func listenOnAvailablePort() (net.Listener, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	return listener, nil
}

// findFileByName recursively searches for a file by name in a directory
func findFileByName(dir, filename string) string {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Base(path) == filename {
			return filepath.SkipDir // Stop walking after finding match
		}
		return nil
	})

	// Second pass to find and return the file
	var foundPath string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if foundPath == "" && !info.IsDir() && filepath.Base(path) == filename {
			foundPath = path
			return filepath.SkipDir
		}
		return nil
	})
	return foundPath
}

// serveMultiQCReport downloads and serves the MultiQC report in a web browser
func serveMultiQCReport(ctxt context.Context, adapter *a.Adapter, multiqcURN string) error {
	// Download the MultiQC artifact
	data, mimeType, err := downloadTarArtifact(ctxt, multiqcURN, adapter)
	if err != nil {
		return fmt.Errorf("failed to download multiqc artifact: %w", err)
	}

	// Verify it's a tar file
	if !isTarFile(mimeType, data) {
		return fmt.Errorf("multiqc artifact is not a tar or tar.gz file (mime-type: %s)", mimeType)
	}

	// Create a temporary directory for the MultiQC report
	tmpDir, err := os.MkdirTemp("", "multiqc-report-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Extract tar contents
	tr, err := openTarReader(data)
	if err != nil {
		return fmt.Errorf("failed to open tar reader: %w", err)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Skip directories
		if header.Typeflag == 53 { // tar.TypeDir
			continue
		}

		// Create file path
		filePath := filepath.Join(tmpDir, filepath.FromSlash(header.Name))
		fileDir := filepath.Dir(filePath)

		// Create directory if needed
		if err := os.MkdirAll(fileDir, 0755); err != nil { // #nosec G301 -- temporary directory
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file
		file, err := os.Create(filePath) // #nosec G304 -- temporary directory
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(file, tr)
		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	// Extract and read MANIFEST.csv to find HTML file
	manifestData, err := extractFileFromTar(data, "MANIFEST.csv")
	if err != nil {
		return fmt.Errorf("MANIFEST.csv not found in multiqc artifact: %w", err)
	}

	// Parse MANIFEST.csv to find HTML files
	reader := csv.NewReader(strings.NewReader(string(manifestData)))
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse MANIFEST.csv: %w", err)
	}

	var htmlFile string
	var htmlFiles []string

	// Skip header row and look for HTML files
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue
		}

		filename := record[0]
		mimetype := record[2]

		// Look for HTML files
		if strings.HasSuffix(strings.ToLower(filename), ".html") && (strings.Contains(mimetype, "text/html") || strings.Contains(mimetype, "html")) {
			htmlFiles = append(htmlFiles, filename)
			// Prefer multiqc.html by name
			if strings.Contains(strings.ToLower(filename), "multiqc.html") {
				htmlFile = filename
			} else if htmlFile == "" {
				htmlFile = filename // Use as fallback
			}
		}
	}

	if htmlFile == "" {
		if len(htmlFiles) > 0 {
			// Use the first HTML file found
			htmlFile = htmlFiles[0]
		} else {
			return fmt.Errorf("no HTML file found in MANIFEST.csv")
		}
	}

	// Verify the file exists after extraction
	htmlFilePath := filepath.Join(tmpDir, htmlFile)
	info, err := os.Stat(htmlFilePath)
	if err != nil {
		return fmt.Errorf("multiqc HTML file not found at %s: %w", htmlFilePath, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("multiqc HTML file is empty: %s", htmlFile)
	}

	// Start a web server on an available port
	listener, err := listenOnAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to start web server: %w", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://localhost:%d", port)

	// Serve the MultiQC report and all extracted files
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Serve the HTML file as root (use full path)
			http.ServeFile(w, r, htmlFilePath)
		} else {
			// Serve other files from the extracted tar contents
			requestPath := strings.TrimPrefix(r.URL.Path, "/")
			filePath := filepath.Join(tmpDir, filepath.FromSlash(requestPath))

			// Security check: ensure the requested file is within tmpDir
			absFilePath, err := filepath.Abs(filePath)
			absTmpDir, err2 := filepath.Abs(tmpDir)
			if err != nil || err2 != nil || !strings.HasPrefix(absFilePath, absTmpDir) {
				http.NotFound(w, r)
				return
			}

			// Try to serve the file
			if _, err := os.Stat(filePath); err == nil { // #nosec G703 -- path already validated above
				http.ServeFile(w, r, filePath)
				return
			}

			// If file not found at exact path, try to find it by searching tmpDir
			foundPath := findFileByName(tmpDir, filepath.Base(requestPath))
			if foundPath != "" {
				http.ServeFile(w, r, foundPath)
				return
			}

			// Not found
			http.NotFound(w, r)
		}
	})

	// Print URL
	fmt.Printf("\n\nMultiQC Report\n")
	fmt.Printf("==============\n")
	fmt.Printf("Report URL: %s\n\n", url)

	// Try to open in browser
	openBrowser(url)

	// Start server (blocking)
	fmt.Printf("Press Ctrl+C to stop the server\n\n")
	if err := http.Serve(listener, nil); err != nil { // #nosec G114 -- local development server
		return fmt.Errorf("web server error: %w", err)
	}

	return nil
}

// openBrowser tries to open the URL in the default browser
func openBrowser(url string) {
	var cmd string
	var args []string

	switch {
	case os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("COLORTERM") != "":
		// Linux with display
		cmd = "xdg-open"
		args = []string{url}
	default:
		// Try macOS
		cmd = "open"
		args = []string{url}
	}

	// Try to open the browser but don't fail if it doesn't work
	// #nosec G204 - URL is safe and constructed by our code
	if err := exec.Command(cmd, args...).Start(); err != nil {
		// Silently ignore - we already printed the URL
		logger.Debug("Could not open browser", log.String("url", url), log.Error(err))
	}
}

// retractNextflowService queries for and retracts the service aspect(s) for a given service ID
func retractNextflowService(ctxt context.Context, serviceID string) error {
	if serviceID == "" {
		return fmt.Errorf("missing service ID")
	}

	adapter := CreateAdapter(true)

	// Query for service aspects with the given entity (service ID)
	selector := sdk.AspectSelector{
		Entity:         serviceID,
		SchemaPrefix:   nf.ServiceSchema,
		ListRequest:    sdk.ListRequest{Limit: 50},
		IncludeContent: false,
	}

	list, _, err := sdk.ListAspect(ctxt, selector, adapter, logger)
	if err != nil {
		return fmt.Errorf("failed to query for service aspects: %w", err)
	}

	if len(list.Items) == 0 {
		return fmt.Errorf("no service aspects found for service ID %s", serviceID)
	}

	// Retract all found aspects
	retractedCount := 0
	var retractErrors []string

	for _, item := range list.Items {
		if item.ID == nil {
			continue
		}
		aspectID := *item.ID

		if _, err := sdk.RetractAspect(ctxt, aspectID, adapter, logger); err != nil {
			retractErrors = append(retractErrors, fmt.Sprintf("  - %s: %v", aspectID, err))
		} else {
			retractedCount++
			if !silent {
				fmt.Printf("Retracted aspect: %s\n", aspectID)
			}
		}
	}

	if len(retractErrors) > 0 {
		return fmt.Errorf("failed to retract %d aspect(s):\n%s", len(retractErrors), strings.Join(retractErrors, "\n"))
	}

	if !silent {
		fmt.Printf("\nSuccessfully retracted %d service aspect(s) for service %s\n", retractedCount, serviceID)
	}

	return nil
}
