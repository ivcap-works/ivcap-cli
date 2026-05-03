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
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
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
	nextflowCmd.AddCommand(nextflowCreateCmd)
	nextflowCmd.AddCommand(nextflowUpdateCmd)
	nextflowCmd.AddCommand(nextflowRunCmd)
	addFileFlag(nextflowCreateCmd, "Path to local tar/tgz containing ivcap.yaml or ivcap-tool.yaml")
	nextflowCreateCmd.Flags().StringVar(&nextflowServiceID, "service-id", "", "Service ID/URN to use for generated service description")
	nextflowCreateCmd.Flags().StringVar(&nextflowCreateFormat, "format", "", "Output format for nextflow create result [json, yaml]")
	cobra.CheckErr(nextflowCreateCmd.MarkFlagRequired("service-id"))

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

var nextflowServiceID string
var nextflowCreateFormat string
var nextflowRunAspectURN string
var nextflowRunWatchFlag bool
var nextflowRunStreamFlag bool
var nextflowRunSamplesheet string
var nextflowJobsJsonFilter string
var nextflowPipelinesJsonFilter string

var (
	nextflowCmd = &cobra.Command{
		Use:   "nextflow",
		Short: "Commands for working with Nextflow-based services",
	}

	nextflowListCmd = &cobra.Command{
		Use:     "list-jobs",
		Aliases: []string{"list-j", "jlist", "list-job", "list-runs"},
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
			return runNextflowCreateOrUpdate(context.Background(), nextflowServiceID)
		},
	}

	nextflowUpdateCmd = &cobra.Command{
		Use:   "update service-id [flags] -f package.tar|package.tgz",
		Short: "Update a Nextflow service definition from a local archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceID := GetHistory(args[0])
			return runNextflowCreateOrUpdate(context.Background(), serviceID)
		},
	}

	nextflowGetJobCmd = &cobra.Command{
		Use:     "get-job [flags] job_id",
		Aliases: []string{"get", "get-run"},
		Short:   "Get status or results of a Nextflow job",
		Long:    "Fetch details about a single Nextflow job without needing the service/pipeline URN",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := GetHistory(args[0])
			ctxt := context.Background()
			return readDisplayJob(ctxt, jobID)
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
			return readDisplayJob(ctxt, jobID)
		},
	}
)

func runNextflowCreateOrUpdate(ctxt context.Context, serviceID string) error {
	if serviceID == "" {
		cobra.CheckErr("Missing service id")
	}
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

	adapter := CreateAdapter(true)
	artifactID, err := nf.UploadArchiveAsArtifact(ctxt, tool.Name, fileName, DEF_CHUNK_SIZE, adapter, silent, logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("while uploading archive as artifact: %v", err))
	}

	svc := nf.BuildServiceDescription(tool, serviceID, artifactID)
	aspectID, err := nf.UpsertServiceDescriptionAspect(ctxt, serviceID, svc, adapter, logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("while publishing service description aspect: %v", err))
	}

	res := &nf.CreateOutput{
		OK:                    true,
		ServiceID:             serviceID,
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
		defer reader.Close()
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
		idx := -1
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
