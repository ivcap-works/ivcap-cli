// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"fmt"
	"net/url"
	"strconv"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"

	api "github.com/ivcap-works/ivcap-core-api/http/secret"
	log "go.uber.org/zap"
)

type ListSecretsRequest struct {
	Page        string
	Limit       int
	OffsetToken string
	Filter      string
}

type GetSecretRequest struct {
	SecretName string
	SecretType string
}

func ListSecrets(ctxt context.Context, host string, req *ListSecretsRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListSecretsRaw(ctxt, host, req, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	if err = pyl.AsType(&list); err != nil {
		return nil, fmt.Errorf("failed to parse list response body: %w", err)
	}
	return &list, nil
}

func ListSecretsRaw(ctxt context.Context, host string, req *ListSecretsRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := "/1/secrets/list"
	q := url.Values{}
	if req.OffsetToken != "" {
		q.Set("offset", req.OffsetToken)
	}
	if req.Filter != "" {
		q.Set("filter", req.Filter)
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	q.Set("limit", strconv.FormatInt(int64(req.Limit), 10))
	path = fmt.Sprintf("%s?%s", path, q.Encode())

	return (*adpt).Get(ctxt, path, logger)
}

func GetSecret(ctxt context.Context, host string, req *GetSecretRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.GetResponseBody, error) {
	pyl, err := GetSecretRaw(ctxt, host, req, adpt, logger)
	if err != nil {
		return nil, err
	}
	var resp api.GetResponseBody
	if err = pyl.AsType(&resp); err != nil {
		return nil, fmt.Errorf("failed to parse secret response body: %w", err)
	}
	return &resp, nil
}

func GetSecretRaw(ctxt context.Context, host string, req *GetSecretRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := "/1/secrets"

	q := url.Values{}
	q.Set("secret-name", req.SecretName)
	if req.SecretType != "" {
		q.Set("secret-type", req.SecretType)
	}
	path = fmt.Sprintf("%s?%s", path, q.Encode())

	return (*adpt).Get(ctxt, path, logger)
}

func SetSecret(ctxt context.Context, host string, req *api.SetRequestBody, adpt *adapter.Adapter, logger *log.Logger) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error marshalling body: %w", err)
	}
	path := "/1/secrets"
	if _, err := (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger); err != nil {
		return fmt.Errorf("failed to set secret via post: %w", err)
	}
	return nil
}
