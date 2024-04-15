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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	api "github.com/ivcap-works/ivcap-core-api/http/order"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

func ListOrders(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListOrdersRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	if err = pyl.AsType(&list); err != nil {
		return nil, fmt.Errorf("failed to parse list response body: %w", err)
	}
	return &list, nil
}

func ListOrdersRaw(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	u, err := createListPath(cmd, orderPath(nil))
	if err != nil {
		return nil, err
	}

	return (*adpt).Get(ctxt, u.String(), logger)
}

/**** CREATE ****/

func CreateOrder(ctxt context.Context, cmd *api.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*api.CreateResponseBody, error) {
	pyl, err := CreateOrderRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var resp api.CreateResponseBody
	if err = pyl.AsType(&resp); err != nil {
		return nil, fmt.Errorf("failed to parse create response body: %w", err)
	}
	return &resp, nil
}

func CreateOrderRaw(ctxt context.Context, cmd *api.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	path := orderPath(nil)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** READ ****/

type ReadOrderRequest struct {
	Id string
}

func ReadOrder(ctxt context.Context, cmd *ReadOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ReadResponseBody, error) {
	pyl, err := ReadOrderRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var order api.ReadResponseBody
	if err = pyl.AsType(&order); err != nil {
		return nil, fmt.Errorf("failed to parse order response body: %w", err)
	}
	return &order, nil
}

func ReadOrderRaw(ctxt context.Context, cmd *ReadOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := orderPath(&cmd.Id)
	return (*adpt).Get(ctxt, path, logger)
}

type LogsRequestBody struct {
	From    int64
	To      int64
	OrderID string
}

func DownloadOrderLog(ctxt context.Context, req *LogsRequestBody, adpt *adapter.Adapter, logger *log.Logger) error {
	path := "/1/orders/" + req.OrderID + "/logs"

	values := url.Values{}
	if req.From != 0 {
		values.Add("from", strconv.FormatInt(req.From, 10))
	}
	if req.To != 0 {
		values.Add("to", strconv.FormatInt(req.To, 10))
	}

	path += "?" + values.Encode()
	handler := func(resp *http.Response, path string, logger *log.Logger) error {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scan download logs error: %w", err)
		}
		return nil
	}

	return (*adpt).GetWithHandler(ctxt, path, nil, handler, logger)
}

func TopOrder(ctxt context.Context, orderID string, adpt *adapter.Adapter, logger *log.Logger) (*api.TopResponseBody, error) {
	pyl, err := TopOrderRaw(ctxt, orderID, adpt, logger)
	if err != nil {
		return nil, err
	}

	var resp api.TopResponseBody
	if err = pyl.AsType(&resp); err != nil {
		return nil, fmt.Errorf("failed to parse create response body: %w", err)
	}
	return &resp, nil
}

func TopOrderRaw(ctxt context.Context, orderID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := "/1/orders/" + orderID + "/top"

	return (*adpt).Get(ctxt, path, logger)
}

/**** UTILS ****/

func orderPath(id *string) string {
	path := "/1/orders"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
