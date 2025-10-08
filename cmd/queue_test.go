// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"testing"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/queue"
)

// testToken string
var queueID string

func TestQueueService(t *testing.T) {
	testCreateQueue(t)
	testEnqueue(t)
	testDequeue(t)
	testReadQueue(t)
	testListQueues(t)
	testDeleteQueue(t)
}

func testCreateQueue(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	createQueueReqBody := []byte(`{
    "name": "end-to-end-test-queue",
    "description": "a queue to test if NATS is running correctly, or not"
  }`)

	pyld, err := a.LoadPayloadFromBytes(createQueueReqBody, false)
	if err != nil {
		t.Fatalf("failed to load payload from 'createQueueReqBody', %v", err)
	}
	var req api.CreateRequestBody
	if err = pyld.AsType(&req); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	res, err := sdk.CreateQueue(context.Background(), &req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	queueID = *res.ID
	t.Logf("queue created: %v", queueID)
}

func testEnqueue(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	schema := "urn:ivcap:schema:queue:message.1"
	payload := `{
    "temperature": "21",
    "location": "Buoy101",
    "timestamp": "2024-05-20T14:30:00Z"
  }`

	req := sdk.ReadQueueRequest{Id: queueID}
	res, err := sdk.Enqueue(context.Background(), &req, schema, payload, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to enqueue message: %v", err)
	}

	t.Logf("message enqueued: %v", *res.ID)
}

func testDequeue(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := sdk.ReadQueueRequest{Id: queueID}
	limit := 1
	res, err := sdk.Dequeue(context.Background(), &req, limit, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to dequeue message: %v", err)
	}
	for _, msg := range res.Messages {
		// Log all attributes of the message
		t.Logf("message dequeued: %v", msg.Content)
	}
}

func testReadQueue(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := sdk.ReadQueueRequest{Id: queueID}
	res, err := sdk.ReadQueue(context.Background(), &req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to read queue: %v", err)
	}
	t.Log("Read queue details:")
	if res.ID != nil {
		t.Logf("\tID: %v", *res.ID)
	}
	if res.Name != nil {
		t.Logf("\tName: %v", *res.Name)
	}
	if res.Description != nil {
		t.Logf("\tDescription: %v", *res.Description)
	}
	if res.TotalMessages != nil {
		t.Logf("\tTotal Messages: %v", *res.TotalMessages)
	}
	if res.Bytes != nil {
		t.Logf("\tBytes: %v", *res.Bytes)
	}
	if res.FirstTime != nil {
		t.Logf("\tFirst Time: %v", *res.FirstTime)
	}
	if res.LastTime != nil {
		t.Logf("\tLast Time: %v", *res.LastTime)
	}
	if res.ConsumerCount != nil {
		t.Logf("\tConsumer Count: %v", *res.ConsumerCount)
	}
	if res.CreatedAt != nil {
		t.Logf("\tCreated At: %v", *res.CreatedAt)
	}
}

func testListQueues(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := sdk.ListRequest{Limit: 5}
	res, err := sdk.ListQueues(context.Background(), &req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to list queues: %v", err)
	}
	t.Log("List of queues:")
	for _, o := range res.Items {
		t.Logf("\tQueue ID '%v' and Name '%v'", *o.ID, *o.Name)
	}
}

func testDeleteQueue(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := sdk.ReadQueueRequest{Id: queueID}
	_, err := sdk.DeleteQueueRaw(context.Background(), &req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to delete queue: %v", err)
	}
	t.Logf("queue deleted: %v", queueID)
}
