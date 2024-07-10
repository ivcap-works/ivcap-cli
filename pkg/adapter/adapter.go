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

// Program to create, update & delete aspect schemas in cayp
// Adapted from https://github.com/maxott/cayp-cli/blob/main/pkg/adapter/adapter.go
package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	log "go.uber.org/zap"
)

type ConnectionCtxt struct {
	URL         string
	AccessToken string
	TimeoutSec  int
	Headers     *map[string]string // default headers
}

func RestAdapter(connCtxt ConnectionCtxt) Adapter {
	return &restAdapter{connCtxt}
}

type IAdapterError interface {
	Error() string
	Path() string
}
type AdapterError struct {
	path string
}

func (e *AdapterError) Path() string { return e.path }

func (e *AdapterError) Error() string { return "Generic cayp adapter error" }

type MissingUrlError struct {
	AdapterError
}

func (e MissingUrlError) Error() string { return "Missing deployment URL" }

type ResourceNotFoundError struct {
	AdapterError
}

func (e ResourceNotFoundError) Error() string { return "Resource not found" }

type UnauthorizedError struct {
	AdapterError
}

func (e *UnauthorizedError) Error() string { return "Unauthorized access" }

type ApiError struct {
	AdapterError
	StatusCode int
	Payload    Payload
}

func (e *ApiError) Error() string {
	if e.Payload != nil && !e.Payload.IsEmpty() {
		return string(e.Payload.AsBytes())
	} else {
		return fmt.Sprintf("%d: %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
}

type ClientError struct {
	AdapterError
	err error
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("while connecting to IVCAP cluster - %s", e.err.Error())
}

type restAdapter struct {
	ctxt ConnectionCtxt
}

func (a *restAdapter) Head(ctxt context.Context, path string, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "HEAD", path, nil, -1, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Get(ctxt context.Context, path string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "GET", path, nil, -1, nil, &a.ctxt, nil, logger)
}

func (a *restAdapter) GetWithHandler(ctxt context.Context, path string, headers *map[string]string, respHandler ResponseHandler, logger *log.Logger) error {
	_, err := Connect(ctxt, "GET", path, nil, -1, headers, &a.ctxt, respHandler, logger)
	return err
}

func (a *restAdapter) Post(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "POST", path, body, length, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) PostWithHandler(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, respHandler ResponseHandler, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "POST", path, body, length, headers, &a.ctxt, respHandler, logger)
}

func (a *restAdapter) PostForm(ctxt context.Context, path string, data neturl.Values, headers *map[string]string, logger *log.Logger) (Payload, error) {
	ed := data.Encode()
	body := strings.NewReader(ed)
	if headers == nil {
		headers = &map[string]string{}
	}
	(*headers)["Content-Type"] = "application/x-www-form-urlencoded"
	return Connect(ctxt, "POST", path, body, int64(len(ed)), headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Put(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "PUT", path, body, length, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Patch(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "PATCH", path, body, length, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Delete(ctxt context.Context, path string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "DELETE", path, nil, -1, nil, &a.ctxt, nil, logger)
}

func (a *restAdapter) SetUrl(url string) {
	a.ctxt.URL = url
}

func (a *restAdapter) GetPath(url string) (path string, err error) {
	if strings.HasPrefix(url, a.ctxt.URL) {
		path = url[len(a.ctxt.URL):]
	} else {
		err = fmt.Errorf("url '%s' is not for this deployment '%s'", url, a.ctxt.URL)
	}
	return
}

func Connect(
	ctxt context.Context,
	method string,
	endpoint string,
	body io.Reader,
	length int64,
	headers *map[string]string,
	connCtxt *ConnectionCtxt,
	respHandler ResponseHandler,
	logger *log.Logger,
) (Payload, error) {
	logger = logger.With(log.String("method", method), log.String("path", endpoint))
	parsedURL, err := parseURL(endpoint, connCtxt)
	if err != nil {
		return nil, err
	}
	logger = logger.With(log.String("url", parsedURL.String()))

	req, err := http.NewRequest(method, parsedURL.String(), body)
	if err != nil {
		logger.Error("Creating http request", log.Error(err))
		return nil, &ClientError{AdapterError{endpoint}, err}
	}
	if length > 0 {
		req.ContentLength = length
	}
	contentType := "application/json"
	if headers != nil {
		if ct, ok := (*headers)["Content-Type"]; ok {
			contentType = ct
		}
	}
	if length > 0 {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Cache-Control", "no-cache")
	if connCtxt.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+connCtxt.AccessToken)
	}
	if connCtxt.Headers != nil {
		for key, val := range *connCtxt.Headers {
			req.Header.Set(key, val)
		}
	}
	if headers != nil {
		for key, val := range *headers {
			if key != "Content-Type" {
				logger.Debug("header", log.String("key", key), log.String("val", val))
				// v := base64.StdEncoding.EncodeToString([]byte(val))
				req.Header.Set(key, val)
			}
		}
	}
	host := req.Header.Get("Host")
	if host != "" {
		req.Host = host
	}
	client := &http.Client{}
	if _, ok := ctxt.Deadline(); !ok {
		client.Timeout = time.Second * time.Duration(connCtxt.TimeoutSec)
	}

	logger.Debug("calling api", log.Reflect("headers", req.Header))
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("HTTP request failed.", log.Error(err), log.Reflect("err2", err))
		return nil, &ClientError{AdapterError{endpoint}, err}
	}
	defer resp.Body.Close()

	if respHandler != nil {
		err := respHandler(resp, endpoint, logger)
		return nil, err
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("Accessing response body failed.", log.Error(err))
		return nil, &ClientError{AdapterError{endpoint}, err}
	}
	logger.Debug("successful reply", log.Int("statusCode", resp.StatusCode),
		log.Int("body-length", len(respBody)), log.Reflect("headers", resp.Header))

	if resp.StatusCode >= 300 {
		if len(respBody) > 0 {
			logger = logger.With(log.ByteString("body", respBody))
		}
		return nil, ProcessErrorResponse(resp, endpoint, ToPayload(respBody, resp, logger), logger)
	}
	return ToPayload(respBody, resp, logger), nil
}

func ProcessErrorResponse(resp *http.Response, path string, pyld Payload, logger *log.Logger) (err error) {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return &ResourceNotFoundError{AdapterError{path}}
	case http.StatusUnauthorized:
		return &UnauthorizedError{AdapterError{path}}
	default:
		logger.Warn("HTTP response", log.Int("statusCode", resp.StatusCode))
		return &ApiError{
			AdapterError: AdapterError{path},
			StatusCode:   resp.StatusCode,
			Payload:      pyld,
		}
	}
}
