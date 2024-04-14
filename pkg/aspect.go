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
	"strconv"
	"strings"

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

func GetAspect(
	ctxt context.Context,
	recordID string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.ReadResponseBody, error) {
	if res, err := GetAspectRaw(ctxt, recordID, adpt, logger); err == nil {
		var response api.ReadResponseBody
		if err := res.AsType(&response); err != nil {
			return nil, err
		}
		return &response, nil
	} else {
		return nil, err
	}

}

func GetAspectRaw(ctxt context.Context, recordID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
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
	ListRequest
	Entity         string
	SchemaPrefix   string
	JsonFilter     *string
	IncludeContent bool
}

func ListAspect(ctxt context.Context,
	selector AspectSelector,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.ListResponseBody, adapter.Payload, error) {
	u, err := createListPath(&selector.ListRequest, aspectPath(nil, adpt))
	if err != nil {
		return nil, nil, err
	}

	q := u.Query()
	if selector.Entity != "" {
		q.Set("entity", selector.Entity)
	}
	if selector.SchemaPrefix != "" {
		q.Set("schema", selector.SchemaPrefix)
	}
	if selector.JsonFilter != nil {
		q.Set("aspect-path", *selector.JsonFilter)
	}
	q.Set("include-content", strconv.FormatBool(selector.IncludeContent))

	u.RawQuery = q.Encode()
	if pyld, err := (*adpt).Get(ctxt, u.String(), logger); err == nil {
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
