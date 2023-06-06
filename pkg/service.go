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
	_ "fmt"
	"net/url"
	"strconv"
	"strings"

	api "github.com/reinventingscience/ivcap-core-api/http/service"

	"github.com/reinventingscience/ivcap-cli/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

type ListServiceRequest struct {
	Offset int
	Limit  int
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

func ListServices(ctxt context.Context, cmd *ListServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListServicesRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	pyl.AsType(&list)
	return &list, nil
}

func ListServicesRaw(ctxt context.Context, cmd *ListServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := servicePath(nil, adpt)

	pa := []string{}
	if cmd.Offset > 0 {
		pa = append(pa, "offset="+url.QueryEscape(strconv.Itoa(cmd.Offset)))
	}
	if cmd.Limit > 0 {
		pa = append(pa, "limit="+url.QueryEscape(strconv.Itoa(cmd.Limit)))
	}
	if len(pa) > 0 {
		path = path + "?" + strings.Join(pa, "&")
	}
	//fmt.Printf("PATH: %s\n", path)
	return (*adpt).Get(ctxt, path, logger)
}

/**** CREATE ****/

//	type CreateServiceRequest struct {
//		Id   string `json:"id"`
//		Name string `json:"name"`
//	}
func CreateServiceRaw(ctxt context.Context, cmd *api.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := servicePath(nil, adpt)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
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

	path := servicePath(&id, adpt)
	if createAnyway {
		path += "?force-create=true"
	}
	return (*adpt).Put(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
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
	path := servicePath(&cmd.Id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

/**** UTILS ****/

func servicePath(id *string, adpt *adapter.Adapter) string {
	path := "/1/services"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
