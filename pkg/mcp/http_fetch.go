// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package mcp

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
)

func fetchURLBytes(ctx context.Context, u string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("url fetch failed: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	mt := resp.Header.Get("Content-Type")
	if mt != "" {
		// drop parameters to keep deterministic
		if m0, _, err := mime.ParseMediaType(mt); err == nil {
			mt = m0
		}
	}
	return b, mt, nil
}

func downloadArtifactBytes(ctx context.Context, dataHref string, adpt *a.Adapter) ([]byte, error) {
	u, err := url.ParseRequestURI(dataHref)
	if err != nil {
		return nil, err
	}
	endpointPath := u.Path
	if u.RawQuery != "" {
		endpointPath = endpointPath + "?" + u.RawQuery
	}
	var out []byte
	// a.ProcessErrorResponse expects a zap logger; use srvCfg.Logger.
	if err := (*adpt).GetWithHandler(ctx, endpointPath, nil, func(resp *http.Response, p string, _ *log.Logger) error {
		if resp.StatusCode >= 300 {
			return a.ProcessErrorResponse(resp, p, nil, srvCfg.Logger)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		out = b
		return nil
	}, srvCfg.Logger); err != nil {
		return nil, err
	}
	return out, nil
}
