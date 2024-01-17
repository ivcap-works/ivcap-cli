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

	api "github.com/ivcap-works/ivcap-core-api/http/aspect"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
)

func AddUpdateAspect(ctxt context.Context, isAdd bool, entity string, schema string, policy string, meta []byte, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	q := []string{}
	if entity != "" {
		q = append(q, fmt.Sprintf("entity=%s", url.QueryEscape(entity)))
	}
	if schema != "" {
		q = append(q, fmt.Sprintf("schema=%s", url.QueryEscape(schema)))
	}
	if policy != "" {
		q = append(q, fmt.Sprintf("policy=%s", url.QueryEscape(policy)))
	}
	path := fmt.Sprintf("%s?%s", aspectPath(nil, adpt), strings.Join(q, "&"))
	if isAdd {
		return (*adpt).Post(ctxt, path, bytes.NewReader(meta), int64(len(meta)), nil, logger)
	} else {
		return (*adpt).Put(ctxt, path, bytes.NewReader(meta), int64(len(meta)), nil, logger)
	}
}

func GetAspect(ctxt context.Context, recordID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := url.PathEscape(recordID)
	path := aspectPath(&id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

func RetractAspect(ctxt context.Context, recordID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := url.PathEscape(recordID)
	path := aspectPath(&id, adpt)
	return (*adpt).Delete(ctxt, path, logger)
}

type AspectSelector struct {
	Entity       string
	SchemaPrefix string
	Page         string
	SimpleFilter *string
	JsonFilter   *string
	Timestamp    *time.Time
	Limit        int
}

func ListAspect(ctxt context.Context,
	selector AspectSelector,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.ListResponseBody, adapter.Payload, error) {
	path := aspectPath(nil, adpt)
	q := url.Values{}
	if selector.Entity != "" {
		q.Set("entity", selector.Entity)
	}
	if selector.SchemaPrefix != "" {
		q.Set("schema", selector.SchemaPrefix)
	}
	if selector.Page != "" {
		q.Set("page", selector.Page)
	}
	if selector.SimpleFilter != nil {
		q.Set("filter", *selector.SimpleFilter)
	}
	if selector.JsonFilter != nil {
		q.Set("aspect-path", *selector.JsonFilter)
	}
	if selector.Timestamp != nil {
		ts := selector.Timestamp.Format(time.RFC3339)
		q.Set("at-time", ts)
	}
	q.Set("limit", fmt.Sprintf("%d", selector.Limit))
	if len(q) > 0 {
		path = fmt.Sprintf("%s?%s", path, q.Encode())
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

func aspectPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/aspects"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
