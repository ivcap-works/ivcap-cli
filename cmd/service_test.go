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

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/service"
)

var (
	createReqBody = []byte(`
{
    "banner": "http://quigleyjakubowski.net/otilia_miller",
    "description": "This service ...",
    "metadata": [
      {
        "name": "Vel cupiditate iure beatae libero.",
        "value": "Culpa nulla facilis voluptatem."
      },
      {
        "name": "Vel cupiditate iure beatae libero.",
        "value": "Culpa nulla facilis voluptatem."
      },
      {
        "name": "Vel cupiditate iure beatae libero.",
        "value": "Culpa nulla facilis voluptatem."
      },
      {
        "name": "Vel cupiditate iure beatae libero.",
        "value": "Culpa nulla facilis voluptatem."
      }
    ],
    "name": "Fire risk for Lot2",
    "parameters": [
      {
        "description": "The name of the region as according to ...",
        "label": "Region Name",
        "name": "region",
        "type": "string"
      },
      {
        "label": "Rainfall/month threshold",
        "name": "threshold",
        "type": "float",
        "unit": "m"
      }
    ],
    "provider-id": "urn:ivcap:provider:0f0e3f57-80f7-4899-9b69-459af2efd789",
    "provider-ref": "service_foo_patch_1",
    "references": [
      {
        "title": "Eius perferendis culpa voluptates fuga dicta.",
        "uri": "http://dach.name/candace.king"
      },
      {
        "title": "Eius perferendis culpa voluptates fuga dicta.",
        "uri": "http://dach.name/candace.king"
      }
    ],
    "tags": [
      "tag1",
      "tag2"
    ],
    "workflow": {
      "argo": "Maxime eius voluptatibus tempore assumenda et qui.",
      "basic": {
        "command": [
          "/bin/sh",
          "-c",
          "echo $PATH"
        ],
        "cpu": {
          "limit": "100m",
          "request": "10m"
        },
        "ephemeral-storage": {
           "limit": "4Gi",
           "request": "2Gi"
        },
        "image": "alpine",
        "memory": {
          "limit": "100Mi",
          "request": "10Mi"
        },
        "shared-memory": "1Gi"
       },
      "opts": "Sed porro.",
      "type": "basic"
    }
  }
`)
)

var (
	adapter   *a.Adapter
	serviceID string
	testToken string
	tlogger   *log.Logger
)

func TestMain(m *testing.M) {
	initConfig()
	ctxt, err := GetContextWithError("", true)
	if err != nil {
		fmt.Printf("Can not get active context, %s\n", err)
		return
	}
	if ctxt.Name != "minikube" {
		fmt.Printf("Unit test should run against minikube, please set to minikube context\n")
		return
	}
	testToken = getAccessToken(true)
	if testToken == "" {
		fmt.Printf("Access token not found\n")
		return
	}

	url := ctxt.URL
	var headers *map[string]string
	if ctxt.Host != "" {
		headers = &(map[string]string{"Host": ctxt.Host})
	}

	adapter, err = NewAdapter(url, testToken, DEFAULT_SERVICE_TIMEOUT_IN_SECONDS, headers)
	if err != nil {
		fmt.Printf("Failed to get adapter: %v\n", err)
		return
	}
	cfg := log.NewDevelopmentConfig()
	cfg.OutputPaths = []string{"stdout"}
	logLevel := zapcore.ErrorLevel
	cfg.Level = log.NewAtomicLevelAt(logLevel)
	tlogger, err = cfg.Build()
	if err != nil {
		fmt.Printf("Failed to create tlogger: %v\n", err)
		return
	}

	os.Exit(m.Run())
}

func TestAll(t *testing.T) {
	testCreateService(t)
	testGetService(t)
	testUpdateService(t)

	testCreateOrder(t)
	testGetOrder(t)

	testAddArtifact(t)
	testGetArtifact(t)
}

func testCreateService(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	pyld, err := a.LoadPayloadFromBytes(createReqBody, false)
	if err != nil {
		t.Fatalf("failed to load payload from 'createReqBody', %v", err)
	}
	var req api.CreateServiceRequestBody
	if err = pyld.AsType(&req); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	res, err := sdk.CreateService(context.Background(), &req, adapter, tlogger)
	if err != nil {
		var apiError *a.ApiError
		if errors.As(err, &apiError) && apiError.StatusCode == http.StatusConflict {
			var payload api.CreateServiceAlreadyCreatedResponseBody
			if err := json.Unmarshal(apiError.Payload.AsBytes(), &payload); err == nil {
				t.Logf("service already exists: %s", *payload.ID)
				serviceID = *payload.ID
			} else {
				t.Fatalf("failed to parse payload :%v", err)
			}
		} else {
			t.Fatalf("failed to create service: %v", err)
		}
	} else {
		if res.ID == nil {
			t.Fatalf("missing ID from create service body: %v", err)
		}

		// set service ID
		serviceID = *res.ID
	}
}

func TestListService(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := &sdk.ListRequest{Limit: 5}
	_, err := sdk.ListServices(context.Background(), req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to list service: %v", err)
	}
}

func testGetService(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	if serviceID == "" {
		t.Skip("service id not set...")
	}

	req := &sdk.ReadServiceRequest{Id: serviceID}
	res, err := sdk.ReadService(context.Background(), req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to read service: %v", err)
	}
	if res.ID == nil {
		t.Fatalf("service id not exists")
	}
	if *res.ID != serviceID {
		t.Fatalf("unexpected updated id: %v, expecting: %s", *res.ID, serviceID)
	}
}

var (
	updateReqBody = []byte(`
	{
		"banner": "http://quigleyjakubowski.net/otilia_miller",
		"description": "This service is updated",
		"metadata": [
		  {
			"name": "Vel cupiditate iure beatae libero.",
			"value": "Culpa nulla facilis voluptatem."
		  },
		  {
			"name": "Vel cupiditate iure beatae libero.",
			"value": "Culpa nulla facilis voluptatem."
		  },
		  {
			"name": "Vel cupiditate iure beatae libero.",
			"value": "Culpa nulla facilis voluptatem."
		  },
		  {
			"name": "Vel cupiditate iure beatae libero.",
			"value": "Culpa nulla facilis voluptatem."
		  }
		],
		"name": "Fire risk for Lot2",
		"parameters": [
		  {
			"description": "The name of the region as according to ...",
			"label": "Region Name",
			"name": "region",
			"type": "string"
		  },
		  {
			"label": "Rainfall/month threshold",
			"name": "threshold",
			"type": "float",
			"unit": "m"
		  },
		  {
			"label": "Rainfall/month threshold",
			"name": "log-version",
			"type": "bool",
			"unary": true,
			"optional": true
		  }
		],
		"provider-id": "urn:ivcap:provider:0f0e3f57-80f7-4899-9b69-459af2efd789",
		"provider-ref": "service_foo_patch_1",
		"references": [
		  {
			"title": "Eius perferendis culpa voluptates fuga dicta.",
			"uri": "http://dach.name/candace.king"
		  },
		  {
			"title": "Eius perferendis culpa voluptates fuga dicta.",
			"uri": "http://dach.name/candace.king"
		  }
		],
		"tags": [
		  "tag1",
		  "tag2"
		],
		"workflow": {
		  "argo": "Maxime eius voluptatibus tempore assumenda et qui.",
		  "basic": {
			"command": [
			  "/usr/local/bin/python",
			  "-c",
			  "import sys,time; print('cmd entry:', sys.argv); time.sleep(60)"
			],
			"cpu": {
			  "limit": "100m",
			  "request": "10m"
			},
			"ephemeral-storage": {
				"limit": "10Gi",
				"request": "2Gi"
			},
			"image": "python",
			"memory": {
			  "limit": "100Mi",
			  "request": "10Mi"
			},
			"shared-memory": "256Mi"
		  },
		  "opts": "Sed porro.",
		  "type": "basic"
		}
	  }
`)
)

func testUpdateService(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	if serviceID == "" {
		t.Skip("service id not set....")
	}

	pyld, err := a.LoadPayloadFromBytes(updateReqBody, false)
	if err != nil {
		t.Fatalf("failed to load payload from 'updateReqBody', %v", err)
	}

	var req api.UpdateRequestBody
	if err = pyld.AsType(&req); err != nil {
		t.Fatalf("failed to unmarshal update request body: %v", err)
	}

	createAnyway := false
	res, err := sdk.UpdateService(context.Background(), serviceID, createAnyway, &req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to update service by id: %s, :%v", serviceID, err)
	}
	if res.ID == nil {
		t.Fatalf("service id not exists")
	}
	if *res.ID != serviceID {
		t.Fatalf("unexpected updated id: %v, expecting: %s", *res.ID, serviceID)
	}

}
