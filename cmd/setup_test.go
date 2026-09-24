// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	adapter   *a.Adapter
	testToken string
	tlogger   *log.Logger
)

func TestMain(m *testing.M) {
	initConfig()

	// Best-effort integration setup: wire up the shared adapter/token only when a
	// suitable local context is configured and already authorised. Pure unit tests
	// (httptest-based) run regardless; integration tests self-skip on testToken == "".
	if ctxt, err := GetContextWithError("", true); err == nil {
		localish := ctxt.Name == "minikube" || ctxt.Name == "docker-desktop" ||
			strings.HasPrefix(ctxt.URL, "http://localhost")
		if localish && IsAuthorised() {
			testToken = getAccessToken(true)
			var headers *map[string]string
			if ctxt.Host != "" {
				headers = &(map[string]string{"Host": ctxt.Host})
			}
			if ad, aerr := NewAdapter(ctxt.URL, testToken, DEFAULT_SERVICE_TIMEOUT_IN_SECONDS, headers); aerr == nil {
				adapter = ad
			} else {
				fmt.Printf("Failed to get adapter: %v\n", aerr)
			}
			cfg := log.NewDevelopmentConfig()
			cfg.OutputPaths = []string{"stdout"}
			cfg.Level = log.NewAtomicLevelAt(zapcore.ErrorLevel)
			if lg, lerr := cfg.Build(); lerr == nil {
				tlogger = lg
			}
		}
	}

	os.Exit(m.Run())
}
