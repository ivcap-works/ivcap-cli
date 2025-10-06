// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	"github.com/r3labs/sse/v2"
)

/**** LIST ****/

type ListServiceRequest struct {
	Offset    int
	Limit     int
	OrderBy   string
	OrderDesc bool
}

// type ListServiceResult struct {
// 	HasMore       bool   `json:"hasMore"`
// 	NextPageToken string `json:"nextPageToken"`
// 	Records       []struct {
// 		Aspects   map[string]interface{} `json:"aspects"`
// 		ID        string                 `json:"id"`
// 		Name      string                 `json:"name"`
// 		SourceTag string                 `json:"sourceTag"`
// 		TenantID  int                    `json:"tenantId"`
// 	} `json:"records"`
// }

func ListServices(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (*ServiceListResponseBody, error) {
	pyl, err := ListServicesRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list ServiceListResponseBody
	if err = pyl.AsType(&list); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}
	return &list, nil
}

func ListServicesRaw(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	u, err := createListPath(cmd, servicePath(nil))
	if err != nil {
		return nil, err
	}
	return (*adpt).Get(ctxt, u.String(), logger)
}

/**** CREATE ****/

//	type CreateServiceRequest struct {
//		Id   string `json:"id"`
//		Name string `json:"name"`
//	}
func CreateServiceRaw(ctxt context.Context, cmd *ServiceCreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := servicePath(nil)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

func CreateService(ctxt context.Context, cmd *ServiceCreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*ServiceCreateResponseBody, error) {
	res, err := CreateServiceRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	var service ServiceCreateResponseBody
	if err := res.AsType(&service); err != nil {
		return nil, fmt.Errorf("failed to decode create service response body; %w", err)
	}

	return &service, nil
}

/**** UPDATE ****/

// type CreateServiceRequest struct {
// 	Id   string `json:"id"`
// 	Name string `json:"name"`
// }

func UpdateServiceRaw(ctxt context.Context, id string, createAnyway bool, cmd *ServiceUpdateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := servicePath(&id)
	if createAnyway {
		path += "?force-create=true"
	}
	return (*adpt).Put(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

func UpdateService(ctxt context.Context, id string, createAnyway bool, cmd *ServiceUpdateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*ServiceUpdateResponseBody, error) {
	res, err := UpdateServiceRaw(ctxt, id, createAnyway, cmd, adpt, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
	}

	var service ServiceUpdateResponseBody
	if err := res.AsType(&service); err != nil {
		return nil, fmt.Errorf("failed to decode update service response body; %w", err)
	}

	return &service, nil
}

/**** READ ****/

type ReadServiceRequest struct {
	Id string
}

func ReadService(ctxt context.Context, cmd *ReadServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (*ServiceReadResponseBody, error) {
	if res, err := ReadServiceRaw(ctxt, cmd, adpt, logger); err == nil {
		var service ServiceReadResponseBody
		if err := res.AsType(&service); err != nil {
			return nil, err
		}
		return &service, nil
	} else {
		return nil, err
	}
}

func ReadServiceRaw(ctxt context.Context, cmd *ReadServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := servicePath(&cmd.Id)
	return (*adpt).Get(ctxt, path, logger)
}

/**** READ JOB ****/

type ReadServiceJobRequest struct {
	ServiceId string
	JobId     string
}

func ReadServiceJob(ctxt context.Context, cmd *ReadServiceJobRequest, adpt *adapter.Adapter, logger *log.Logger) (*JobReadResponseBody, adapter.Payload, error) {
	if res, err := ReadServiceJobRaw(ctxt, cmd, adpt, logger); err == nil {
		var service JobReadResponseBody
		if err := res.AsType(&service); err != nil {
			return nil, nil, err
		}
		return &service, res, nil
	} else {
		return nil, nil, err
	}
}

func ReadServiceJobRaw(ctxt context.Context, cmd *ReadServiceJobRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := serviceJobPath(cmd.ServiceId, &cmd.JobId)
	return (*adpt).Get(ctxt, path, logger)
}

/**** CREATE JOB ****/

type JobCreateT struct {
	JobID      string  `json:"job-id"`
	ServiceID  string  `json:"service-id,omitempty"`
	RetryLater float64 `json:"retry-later"`
}

func CreateServiceJobRaw(ctxt context.Context, serviceId string, pyld adapter.Payload, timeout int, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, *JobCreateT, error) {
	path := serviceJobPath(serviceId, nil)
	body, len := pyld.AsReader()
	headers := &map[string]string{
		"Content-Type": pyld.ContentType(),
		"Timeout":      fmt.Sprintf("%d", timeout),
	}
	res, err := (*adpt).Post(ctxt, path, body, len, headers, logger)
	if err != nil {
		return nil, nil, err
	}
	if res.StatusCode() == 202 {
		var jobCreate JobCreateT
		if err := res.AsType(&jobCreate); err != nil {
			return nil, nil, err
		}
		// jobCreate.ServiceID = serviceID
		return res, &jobCreate, nil
	}
	return res, nil, nil
}

/**** JOB EVENTS ****/

func GetJobEvents(ctxt context.Context, serviceId string, jobId string, lastEventID *string, onEvent func(*sse.Event), adpt *adapter.Adapter, logger *log.Logger) error {
	path := serviceJobPath(serviceId, &jobId) + "/events"
	return (*adpt).GetSSE(ctxt, path, lastEventID, onEvent, nil, logger)
}

/**** UTILS ****/

func servicePath(id *string) string {
	path := "/1/services2"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}

func serviceJobPath(serviceID string, jobID *string) string {
	path := fmt.Sprintf("/1/services2/%s/jobs", serviceID)
	if jobID != nil {
		path = path + "/" + *jobID
	}
	return path
}
