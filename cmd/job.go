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
	"bytes"
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
	"github.com/r3labs/sse/v2"
	"github.com/spf13/cobra"
)

const JOB_SCHEMA = "urn:ivcap:schema:job.2"

const CREATE_FROM_ASPECT = `{
  "$schema": "urn:ivcap:schema:job.input.1",
	"request-aspect": "%s",
	"service": "%s"
}`

func init() {
	rootCmd.AddCommand(jobCmd)

	// LIST
	jobCmd.AddCommand(listJobCmd)
	jobCmd.Flags().StringVarP(&jobsJsonFilter, "content-path", "c", "", "json path filter on jobs's content ('$.images[*] ? (@.size > 10000)')")

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
}

var (
	jobsJsonFilter string
	aspectURN      string
	watchFlag      bool
	streamFlag     bool
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
			return readDisplayJob(recordID)
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
					cobra.CheckErr(fmt.Sprintf("While reading job file '%s' - %s", fileName, err))
				}
			}
			if aspectURN != "" {
				j := fmt.Sprintf(CREATE_FROM_ASPECT, aspectURN, serviceID)
				if pyld, err = a.LoadPayloadFromBytes([]byte(j), false); err != nil {
					cobra.CheckErr(fmt.Sprintf("While reading job file '%s' - %s", fileName, err))
				}
			}
			res, err := sdk.CreateServiceJobRaw(ctxt, serviceID, pyld, 0, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			if res.StatusCode() == 202 {
				return waitForResult(ctxt, res, serviceID)
			}
			reply, err := res.AsObject()
			if err != nil {
				return err
			}
			jobID, ok := reply["job-id"].(string)
			if !ok {
				cobra.CheckErr("Cannot find job ID in response")
			}
			return readDisplayJob(jobID) // a.ReplyPrinter(res, outputFormat == "yaml")
		},
	}
)

type JobCreateT struct {
	JobID      string  `json:"job-id"`
	ServiceID  string  `json:"service-id,omitempty"`
	RetryLater float64 `json:"retry-later"`
}

func waitForResult(ctxt context.Context, res a.Payload, serviceID string) error {
	var jobCreate JobCreateT
	if err := res.AsType(&jobCreate); err != nil {
		return err
	}
	jobCreate.ServiceID = serviceID
	if streamFlag {
		return streamJobResults(ctxt, &jobCreate)
	}
	wait := 2
	if !watchFlag {
		wait = int(math.Min(jobCreate.RetryLater, float64(timeout)))
	}
	logger.Info("Job created", log.String("job-id", jobCreate.JobID), log.Int("waiting [sec]", wait))

	jobID := jobCreate.JobID
	done := false
	for !done {
		time.Sleep(time.Duration(wait) * time.Second)
		job, pyld, err := readJob(jobID)
		if err != nil {
			return err
		}
		status := "?"
		if job.Status != nil {
			status = *job.Status
		}
		done = !watchFlag || !(status == "?" || status == "scheduled" || status == "executing")
		if done {
			return displayJob(job, pyld)
		}
	}

	return readDisplayJob(jobCreate.JobID)
}

func streamJobResults(ctxt context.Context, jobCreate *JobCreateT) error {
	onEvent := func(msg *sse.Event) {
		var out bytes.Buffer
		if err := json.Indent(&out, msg.Data, "", "  "); err == nil {
			fmt.Println("---------")
			s := out.String()
			fmt.Println(s)
		}
	}
	err := sdk.GetJobEvents(ctxt, jobCreate.ServiceID, jobCreate.JobID, nil, onEvent, CreateAdapter(true), logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("While watching events for job '%s' - %s", jobCreate.JobID, err))
	}
	fmt.Println("---------")
	return readDisplayJob(jobCreate.JobID)
}

func readDisplayJob(jobID string) error {
	job, pyld, err := readJob(jobID)
	if err != nil {
		return err
	}
	return displayJob(job, pyld)
}

func displayJob(job *sdk.JobReadResponseBody, pyld a.Payload) error {
	switch outputFormat {
	case "json", "yaml":
		return a.ReplyPrinter(pyld, outputFormat == "yaml")
	default:
		printJob(job, false)
	}
	return nil
}

func readJob(jobID string) (*sdk.JobReadResponseBody, a.Payload, error) {
	selector := sdk.AspectSelector{
		Entity:         jobID,
		SchemaPrefix:   JOB_SCHEMA,
		IncludeContent: true,
	}
	ctxt := context.Background()
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
		return nil, nil, err
	}
	req := &sdk.ReadServiceJobRequest{ServiceId: serviceId, JobId: jobID}
	job, pyld, err := sdk.ReadServiceJob(context.Background(), req, CreateAdapter(true), logger)
	return job, pyld, err
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

func printJob(job *sdk.JobReadResponseBody, wide bool) {

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false

	rows := []table.Row{}
	if job.Name != nil {
		rows = append(rows, table.Row{"Name", safeString(job.Name)}, table.Row{"", ""})
	}
	id := fmt.Sprintf("%s (%s)", *job.ID, MakeHistory(job.ID))
	var service string
	// API should be updated to return service name as well
	if job.Service != nil {
		service = fmt.Sprintf("%s (%s)", *job.Service, MakeHistory(job.Service))
	}
	rows = append(rows,
		table.Row{"ID", id},
		table.Row{"Status", safeString(job.Status)},
		table.Row{"Started At", safeDate(job.StartedAt, false)},
	)

	if job.FinishedAt != nil {
		rows = append(rows,
			table.Row{"Finished At", safeDate(job.FinishedAt, false)},
		)
	}

	rows = append(rows,
		table.Row{"Service", service},
		table.Row{"Policy", safeString(job.Policy)},
		table.Row{"Account", safeString(job.Account)},
	)

	if job.ResultContentType != nil {
		ct := *job.ResultContentType
		rows = append(rows,
			table.Row{"", ""},
			table.Row{"Result-Type", ct},
		)

		if ct == "application/json" || strings.HasPrefix(ct, "application/vnd.") {
			content, err := a.ToString(job.ResultContent, false)
			if err != nil {
				fmt.Printf("ERROR: cannot print job result - %v\n", err)
				return
			}
			rows = append(rows, table.Row{"Result", content})
		} else {
			rows = append(rows, table.Row{"Result", ".... cannot print"})
		}
	}

	tw.AppendRows(rows)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 100, WidthMaxEnforcer: WrapSoftSoft},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func findNextJobPage(links []*sdk.LinkTResponseBody) *string {
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
