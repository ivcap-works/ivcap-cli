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

// Adapted from https://github.com/maxott/magda-cli/blob/main/pkg/adapter/adapter.go
package adapter

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/r3labs/sse/v2"
	log "go.uber.org/zap"
)

type ResponseHandler func(response *http.Response, path string, logger *log.Logger) (err error)

type Adapter interface {
	Head(ctxt context.Context, path string, headers *map[string]string, logger *log.Logger) (Payload, error)
	Get(ctxt context.Context, path string, logger *log.Logger) (Payload, error)
	GetWithHandler(ctxt context.Context, path string, headers *map[string]string, respHandler ResponseHandler, logger *log.Logger) error
	Post(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error)
	PostWithHandler(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, respHandler ResponseHandler, logger *log.Logger) (Payload, error)
	PostForm(ctxt context.Context, path string, data url.Values, headers *map[string]string, logger *log.Logger) (Payload, error)
	Put(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error)
	Patch(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error)
	Delete(ctxt context.Context, path string, logger *log.Logger) (Payload, error)
	GetSSE(ctxt context.Context, path string, lastEventID *string, onEvent func(*sse.Event), headers *map[string]string, logger *log.Logger) error
	SetUrl(url string)
	GetPath(url string) (path string, err error)
}

type Payload interface {
	// IsObject() bool
	AsType(r interface{}) error
	AsObject() (map[string]interface{}, error)
	AsArray() ([]interface{}, error)
	AsBytes() []byte
	AsReader() (io.Reader, int64)
	IsEmpty() bool
	Header(key string) string
	ContentType() string
	StatusCode() int
}
