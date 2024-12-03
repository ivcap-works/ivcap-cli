// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	ctxt, err := GetContextWithError("", true)
	if err != nil {
		fmt.Printf("Can not get active context, %s\n", err)
		return
	}
	if ctxt.Name != "minikube" && ctxt.Name != "docker-desktop" && !strings.HasPrefix(ctxt.URL, "http://localhost") {
		fmt.Printf("Unit test should run against minikube, please set to minikube context\n")
		return
	}
	testToken = getAccessToken(true)
	if testToken == "" {
		fmt.Printf("Access token not found\n")
		return
	}

	url := ctxt.URL
	var headers *map[string]string
	if ctxt.Host != "" {
		headers = &(map[string]string{"Host": ctxt.Host})
	}

	adapter, err = NewAdapter(url, testToken, DEFAULT_SERVICE_TIMEOUT_IN_SECONDS, headers)
	if err != nil {
		fmt.Printf("Failed to get adapter: %v\n", err)
		return
	}
	cfg := log.NewDevelopmentConfig()
	cfg.OutputPaths = []string{"stdout"}
	logLevel := zapcore.ErrorLevel
	cfg.Level = log.NewAtomicLevelAt(logLevel)
	tlogger, err = cfg.Build()
	if err != nil {
		fmt.Printf("Failed to create tlogger: %v\n", err)
		return
	}

	os.Exit(m.Run())
}
