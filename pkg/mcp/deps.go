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

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// ---- Dependency injection seams (primarily for tests) --------------------------------------

// allow test stubbing
var (
	listAspectFn          = sdk.ListAspect
	getAspectRawFn        = sdk.GetAspectRaw
	addUpdateAspectFn     = sdk.AddUpdateAspect
	listServicesRawFn     = sdk.ListServicesRaw
	createServiceJobRawFn = sdk.CreateServiceJobRaw
	createArtifactFn      = sdk.CreateArtifact
	uploadArtifactFn      = sdk.UploadArtifact
	readArtifactFn        = sdk.ReadArtifact
)

// allow test stubbing (network helpers)
var (
	fetchURLBytesFn         = fetchURLBytes
	downloadArtifactBytesFn = downloadArtifactBytes
)

func createAdapter(timeoutSec int) (*a.Adapter, error) {
	if srvCfg.CreateAdapter == nil {
		return nil, fmt.Errorf("mcp: missing CreateAdapter in config")
	}
	return srvCfg.CreateAdapter(timeoutSec)
}

func withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if srvCfg.TimeoutSec <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, seconds(srvCfg.TimeoutSec))
}
