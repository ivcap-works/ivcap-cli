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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	nf "github.com/ivcap-works/ivcap-cli/pkg/nextflow"
)

func init() {
	rootCmd.AddCommand(nextflowCmd)

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
	addFileFlag(nextflowRunCmd, "Path to job input file")
	addInputFormatFlag(nextflowRunCmd)
	nextflowRunCmd.Flags().StringVarP(&nextflowRunAspectURN, "aspect", "a", "", "URN of aspect containing job parameters")
	nextflowRunCmd.Flags().BoolVar(&nextflowRunWatchFlag, "watch", false, "if set, watch the job until it is finished")
	nextflowRunCmd.Flags().BoolVar(&nextflowRunStreamFlag, "stream", false, "if set, print job related events to stdout")
}

var nextflowServiceID string
var nextflowCreateFormat string
var nextflowRunAspectURN string
var nextflowRunWatchFlag bool
var nextflowRunStreamFlag bool

var (
	nextflowCmd = &cobra.Command{
		Use:   "nextflow",
		Short: "Commands for working with Nextflow-based services",
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

	nextflowRunCmd = &cobra.Command{
		Use:   "run [flags] service-id -f job-input|- -a aspect-urn --watch --stream",
		Short: "Alias for 'ivcap job create'",
		Long:  "Alias for 'ivcap job create' (creates a job for a given service ID with provided input).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			serviceID := GetHistory(args[0])
			if fileName == "" && nextflowRunAspectURN == "" {
				cobra.CheckErr("Missing parameter file '-f job-file|-' or '-a aspectURN'")
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
		fmt.Fprintln(os.Stdout, "Nextflow service created successfully")
		fmt.Fprintf(os.Stdout, "  service:  %s\n", out.ServiceID)
		fmt.Fprintf(os.Stdout, "  pipeline: %s\n", out.PipelineArtifactURN)
		if out.ServiceAspectRecordID != "" {
			fmt.Fprintf(os.Stdout, "  aspect:   %s\n", out.ServiceAspectRecordID)
		}
		return nil
	case "json":
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	case "yaml":
		b, err := yaml.Marshal(out)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	default:
		return fmt.Errorf("unsupported --format %q (expected json|yaml)", nextflowCreateFormat)
	}
}
