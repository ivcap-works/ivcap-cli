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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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

// realConfigFile is the user's actual config.yaml, resolved before any test can
// redirect the config dir. It is only ever read, never written, by the tests.
var realConfigFile string

// useIsolatedConfigDir points the CLI at a fresh temp config dir and verifies
// the redirect took effect, so a test can never write to the user's real config.
func useIsolatedConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(CONFIG_DIR_ENV, dir)

	got := GetConfigFilePath()
	if filepath.Dir(got) != dir {
		t.Fatalf("config path %q is not inside the isolated dir %q", got, dir)
	}
	if realConfigFile != "" && got == realConfigFile {
		t.Fatalf("refusing to run: config path %q is the user's real config", got)
	}
	return dir
}

func TestMain(m *testing.M) {
	if os.Getenv(CONFIG_DIR_ENV) == "" {
		realConfigFile = GetConfigFilePath()
	}
	realBefore, _ := os.ReadFile(realConfigFile) // nil if absent
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

	code := m.Run()

	// Safety net: fail the run loudly if anything modified the real config.
	if realConfigFile != "" {
		if after, _ := os.ReadFile(realConfigFile); !bytes.Equal(realBefore, after) {
			fmt.Fprintf(os.Stderr, "FAIL: tests modified the user's real config %s\n", realConfigFile)
			code = 1
		}
	}
	os.Exit(code)
}

func TestConfigDirEnvOverride(t *testing.T) {
	dir := useIsolatedConfigDir(t)
	if got := GetConfigDir(false); got != dir {
		t.Fatalf("GetConfigDir = %q, want %q", got, dir)
	}
	if got, want := GetConfigFilePath(), filepath.Join(dir, CONFIG_FILE_NAME); got != want {
		t.Fatalf("GetConfigFilePath = %q, want %q", got, want)
	}
}

func TestConfigDirDefaultsToUserConfigDir(t *testing.T) {
	t.Setenv(CONFIG_DIR_ENV, "")
	want, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no user config dir")
	}
	if got := GetConfigDir(false); got != filepath.Join(want, CONFIG_FILE_DIR) {
		t.Fatalf("GetConfigDir = %q", got)
	}
}
