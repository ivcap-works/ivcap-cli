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

	api "github.com/reinventingscience/ivcap-core-api/http/metadata"

	"github.com/reinventingscience/ivcap-client/pkg/adapter"
	log "go.uber.org/zap"
)

func AddUpdateMetadata(ctxt context.Context, isAdd bool, entity string, schema string, meta []byte, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	q := []string{}
	if entity != "" {
		q = append(q, fmt.Sprintf("entity-id=%s", url.QueryEscape(entity)))
	}
	if schema != "" {
		q = append(q, fmt.Sprintf("schema=%s", url.QueryEscape(schema)))
	}
	path := fmt.Sprintf("%s?%s", metadataPath(nil, adpt), strings.Join(q, "&"))
	if isAdd {
		return (*adpt).Post(ctxt, path, bytes.NewReader(meta), int64(len(meta)), nil, logger)
	} else {
		return (*adpt).Put(ctxt, path, bytes.NewReader(meta), int64(len(meta)), nil, logger)
	}
}

func GetMetadata(ctxt context.Context, recordID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := url.PathEscape(recordID)
	path := metadataPath(&id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

func RevokeMetadata(ctxt context.Context, recordID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := url.PathEscape(recordID)
	path := metadataPath(&id, adpt)
	return (*adpt).Delete(ctxt, path, logger)
}

func ListMetadata(ctxt context.Context,
	entity string,
	schemaPrefix string,
	timestamp *time.Time,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.ListResponseBody, adapter.Payload, error) {
	path := metadataPath(nil, adpt)
	q := make([]string, 0)
	if entity != "" {
		q = append(q, fmt.Sprintf("entity-id=%s", url.QueryEscape(entity)))
	}
	if schemaPrefix != "" {
		q = append(q, fmt.Sprintf("schema=%s", url.QueryEscape(schemaPrefix)))
	}
	if timestamp != nil {
		ts := timestamp.Format(time.RFC3339)
		q = append(q, fmt.Sprintf("at-time=%s", url.QueryEscape(ts)))
	}
	if len(q) > 0 {
		path = fmt.Sprintf("%s?%s", path, strings.Join(q, "&"))
	}
	if pyld, err := (*adpt).Get(ctxt, path, logger); err == nil {
		var list api.ListResponseBody
		if err := pyld.AsType(&list); err == nil {
			return &list, pyld, nil
		} else {
			return nil, nil, err
		}
	} else {
		return nil, nil, err
	}
}

/**** UTILS ****/

func metadataPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/metadata"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
