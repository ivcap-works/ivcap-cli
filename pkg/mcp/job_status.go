// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO)
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

package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// MCP tool: job_status
//
// Check the status or retrieve the result of a previously submitted long-running job.

type jobStatusArgs struct {
	JobID     string `json:"job_id"`
	ServiceID string `json:"service_id,omitempty"`
}

func addJobStatusTool(s *server.MCPServer) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":     map[string]any{"type": "string", "description": "Job URN/ID to check status for."},
			"service_id": map[string]any{"type": "string", "description": "Optional service URN/ID (improves query performance if provided)."},
		},
		"required": []any{"job_id"},
	}

	tool := mcp.NewToolWithRawSchema(
		"job_status",
		"Check the status or retrieve the result of a previously submitted long-running job.",
		MapToRaw(schema),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		b, err := a.JsonPayloadFromAny(args, srvCfg.Logger)
		if err != nil {
			return nil, err
		}
		var parsed jobStatusArgs
		if err := b.AsType(&parsed); err != nil {
			return nil, err
		}
		if parsed.JobID == "" {
			return nil, fmt.Errorf("missing job_id")
		}

		adpt, err := createAdapter(srvCfg.TimeoutSec)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}
		ctxt, cancel := withTimeout(ctx)
		defer cancel()

		// Attempt to read job status
		jobReq := &sdk.ReadServiceJobRequest{
			JobId:     parsed.JobID,
			ServiceId: parsed.ServiceID,
		}
		job, pyld, err := sdk.ReadServiceJob(ctxt, jobReq, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return NewLoginRequiredResult(), nil
			}
			return nil, err
		}

		status := "unknown"
		if job != nil && job.Status != nil {
			status = *job.Status
		}

		// Check if job is done
		done := status != "unknown" && status != "scheduled" && status != "executing"

		if done {
			// Job completed - return the result
			o, err := pyld.AsObject()
			if err != nil {
				return nil, err
			}
			// Job responses wrap the actual result payload in 'result-content'.
			if rc, ok := o["result-content"].(map[string]any); ok {
				return mcp.NewToolResultJSON(map[string]any{
					"status": status,
					"result": rc,
					"_meta": map[string]any{
						"job_id": parsed.JobID,
						"status": status,
					},
				})
			}
			// Fallback if result-content is missing
			return mcp.NewToolResultJSON(map[string]any{
				"status":  status,
				"message": "Job completed but result-content unavailable.",
				"_meta": map[string]any{
					"job_id": parsed.JobID,
					"status": status,
				},
			})
		}

		// Job still running
		pollAfter := estimatePollInterval(status)
		return mcp.NewToolResultJSON(map[string]any{
			"status":  status,
			"message": fmt.Sprintf("Job is still %s. Try again in %d seconds.", status, pollAfter),
			"_meta": map[string]any{
				"job_id":             parsed.JobID,
				"status":             status,
				"poll_after_seconds": pollAfter,
			},
		})
	}

	s.AddTool(tool, handler)
}

// estimatePollInterval returns a suggested poll interval based on job status
func estimatePollInterval(status string) int {
	switch status {
	case "scheduled":
		return 10 // Check more frequently for scheduled jobs
	case "executing":
		return 30 // Less frequent for executing jobs
	default:
		return 30
	}
}

// JobWaitResult encapsulates the result of waiting for a job with timeout
type JobWaitResult struct {
	JobID      string
	Status     string
	Result     map[string]any
	IsComplete bool
	Message    string
}

// waitForServiceJobOptimistic attempts to wait for a job with an optimistic timeout.
// If the job completes within maxWaitSeconds, it returns the result.
// Otherwise, it returns partial status indicating the job is still running.
func waitForServiceJobOptimistic(
	ctx context.Context,
	serviceID string,
	jobCreate *sdk.JobCreateT,
	adpt *a.Adapter,
	maxWaitSeconds int,
) (*JobWaitResult, error) {
	jobID := jobCreate.JobID
	if jobCreate.ServiceID != "" {
		serviceID = jobCreate.ServiceID
	}

	// Use shorter intervals for optimistic wait
	pollInterval := 2 * time.Second
	maxWait := time.Duration(maxWaitSeconds) * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		job, pyld, err := sdk.ReadServiceJob(ctx, &sdk.ReadServiceJobRequest{
			ServiceId: serviceID,
			JobId:     jobID,
		}, adpt, srvCfg.Logger)
		if err != nil {
			if isAuthFailure(err) {
				return nil, ErrLoginRequired
			}
			return nil, err
		}

		status := "unknown"
		if job != nil && job.Status != nil {
			status = *job.Status
		}

		done := status != "unknown" && status != "scheduled" && status != "executing"
		if done {
			// Job completed within timeout - return result
			o, err := pyld.AsObject()
			if err != nil {
				return nil, err
			}
			// Job responses wrap the actual result payload in 'result-content'.
			if rc, ok := o["result-content"].(map[string]any); ok {
				return &JobWaitResult{
					JobID:      jobID,
					Status:     status,
					Result:     rc,
					IsComplete: true,
					Message:    "Job completed successfully.",
				}, nil
			}
			return &JobWaitResult{
				JobID:      jobID,
				Status:     status,
				Result:     o,
				IsComplete: true,
				Message:    "Job completed.",
			}, nil
		}

		// Still running, continue polling
	}

	// Timeout reached - job still running
	// Get latest status
	job, _, _ := sdk.ReadServiceJob(ctx, &sdk.ReadServiceJobRequest{
		ServiceId: serviceID,
		JobId:     jobID,
	}, adpt, srvCfg.Logger)

	status := "executing"
	if job != nil && job.Status != nil {
		status = *job.Status
	}

	return &JobWaitResult{
		JobID:      jobID,
		Status:     status,
		Result:     nil,
		IsComplete: false,
		Message:    fmt.Sprintf("Job still %s. Estimated wait: 2-3 minutes.", status),
	}, nil
}
