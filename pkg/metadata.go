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
	"strings"
	"time"

	"fmt"
	"net/url"

	"github.com/reinventingscience/ivcap-client/pkg/adapter"
	log "go.uber.org/zap"
)

func AddMetadata(ctxt context.Context, entity string, schema string, meta []byte, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := fmt.Sprintf("%s/%s", url.PathEscape(entity), url.PathEscape(schema))
	path := metadataPath(&id, adpt)
	return (*adpt).Put(ctxt, path, bytes.NewReader(meta), int64(len(meta)), nil, logger)
}

func GetMetadata(ctxt context.Context, entity string, schemas string, timestamp *time.Time, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := url.PathEscape(entity)
	path := metadataPath(&id, adpt)
	q := make([]string, 0)
	if schemas != "" {
		q = append(q, fmt.Sprintf("$schema_filter=%s", url.QueryEscape(schemas)))
	}
	if timestamp != nil {
		ts := timestamp.Format(time.RFC3339)
		q = append(q, fmt.Sprintf("$at-time=%s", url.QueryEscape(ts)))
	}
	if len(q) > 0 {
		path = fmt.Sprintf("%s?%s", path, strings.Join(q, "&"))
	}
	return (*adpt).Get(ctxt, path, logger)
}

func RevokeMetadata(ctxt context.Context, recordID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := url.PathEscape(recordID)
	path := metadataPath(&id, adpt)
	return (*adpt).Delete(ctxt, path, logger)
}

/**** UTILS ****/

func metadataPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/meta"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
