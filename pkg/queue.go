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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"

	api "github.com/ivcap-works/ivcap-core-api/http/queue"
	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

/**** LIST ****/
func ListQueues(
	ctxt context.Context,
	cmd *ListRequest,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.ListResponseBody, error) {
	res, err := ListQueuesRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}

	var queues api.ListResponseBody
	if err := res.AsType(&queues); err != nil {
		return nil, err
	}

	return &queues, nil
}

func ListQueuesRaw(
	ctxt context.Context,
	cmd *ListRequest,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	u, err := createListPath(cmd, queuePath(nil))
	if err != nil {
		return nil, err
	}

	return (*adpt).Get(ctxt, u.String(), logger)
}

/**** READ ****/
type ReadQueueRequest struct {
	Id string
}

func ReadQueue(
	ctxt context.Context,
	cmd *ReadQueueRequest,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.ReadResponseBody, error) {
	res, err := ReadQueueRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}

	var queue api.ReadResponseBody
	if err := res.AsType(&queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

func ReadQueueRaw(
	ctxt context.Context,
	cmd *ReadQueueRequest,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := queuePath(&cmd.Id)
	return (*adpt).Get(ctxt, path, logger)
}

/**** CREATE ****/
func CreateQueue(
	ctxt context.Context,
	cmd *api.CreateRequestBody,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.CreateResponseBody, error) {
	res, err := CreateQueueRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}

	var queue api.CreateResponseBody
	if err := res.AsType(&queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

func CreateQueueRaw(
	ctxt context.Context,
	cmd *api.CreateRequestBody,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}

	path := queuePath(nil)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** DELETE ****/
func DeleteQueueRaw(
	ctx context.Context,
	cmd *ReadQueueRequest,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := queuePath(&cmd.Id)
	return (*adpt).Delete(ctx, path, logger)
}

/**** ENQUEUE ****/
func Enqueue(
	ctx context.Context,
	cmd *ReadQueueRequest,
	schema string,
	message string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.EnqueueResponseBody, error) {
	res, err := EnqueueRaw(ctx, cmd, schema, message, adpt, logger)
	if err != nil {
		return nil, err
	}

	var queue api.EnqueueResponseBody
	if err := res.AsType(&queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

func EnqueueRaw(
	ctx context.Context,
	cmd *ReadQueueRequest,
	schema string,
	message string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := queuePath(&cmd.Id) + "/messages"
	if schema != "" {
		path += "?schema=" + schema
	}

	payload := []byte(message)
	return (*adpt).Post(ctx, path, bytes.NewReader(payload), int64(len(payload)), nil, logger)
}

/**** DEQUEUE ****/
func Dequeue(
	ctx context.Context,
	cmd *ReadQueueRequest,
	limit int,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.DequeueResponseBody, error) {
	res, err := DequeueRaw(ctx, cmd, limit, adpt, logger)
	if err != nil {
		return nil, err
	}

	var queue api.DequeueResponseBody
	if err := res.AsType(&queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

func DequeueRaw(
	ctx context.Context,
	cmd *ReadQueueRequest,
	limit int,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	logger.Debug("Dequeue request", log.String("queue", cmd.Id), log.Int("limit", limit))

	limit = max(limit, 1)
	path := queuePath(&cmd.Id) + "/messages?limit=" + strconv.Itoa(limit)
	logger.Debug("Dequeue path", log.String("path", path))
	return (*adpt).Get(ctx, path, logger)
}

/**** UTILS ****/

func queuePath(id *string) string {
	path := "/1/queues"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
