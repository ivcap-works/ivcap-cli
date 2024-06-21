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
	api "github.com/ivcap-works/ivcap-core-api/http/service"
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

func ListServices(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListServicesRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
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
	// path := servicePath(nil)
	// u, err := url.Parse(path)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to parse path %s to url: %w", path, err)
	// }

	// query := u.Query()
	// if cmd.Offset > 0 {
	// 	query.Set("offset", strconv.FormatInt(int64(cmd.Offset), 10))
	// }
	// if cmd.Limit > 0 {
	// 	query.Set("limit", strconv.FormatInt(int64(cmd.Limit), 10))
	// }
	// query.Set("order-by", cmd.OrderBy)
	// query.Set("order-desc", strconv.FormatBool(cmd.OrderDesc))

	// u.RawQuery = query.Encode()

	// return (*adpt).Get(ctxt, u.String(), logger)
}

/**** CREATE ****/

//	type CreateServiceRequest struct {
//		Id   string `json:"id"`
//		Name string `json:"name"`
//	}
func CreateServiceRaw(ctxt context.Context, cmd *api.CreateServiceRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := servicePath(nil)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

func CreateService(ctxt context.Context, cmd *api.CreateServiceRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*api.CreateServiceResponseBody, error) {
	res, err := CreateServiceRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	var service api.CreateServiceResponseBody
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

func UpdateServiceRaw(ctxt context.Context, id string, createAnyway bool, cmd *api.UpdateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
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

func UpdateService(ctxt context.Context, id string, createAnyway bool, cmd *api.UpdateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*api.UpdateResponseBody, error) {
	res, err := UpdateServiceRaw(ctxt, id, createAnyway, cmd, adpt, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
	}

	var service api.UpdateResponseBody
	if err := res.AsType(&service); err != nil {
		return nil, fmt.Errorf("failed to decode update service response body; %w", err)
	}

	return &service, nil
}

/**** READ ****/

type ReadServiceRequest struct {
	Id string
}

func ReadService(ctxt context.Context, cmd *ReadServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ReadResponseBody, error) {
	if res, err := ReadServiceRaw(ctxt, cmd, adpt, logger); err == nil {
		var service api.ReadResponseBody
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

/**** UTILS ****/

func servicePath(id *string) string {
	path := "/1/services"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
