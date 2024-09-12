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

package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ENV_PREFIX = "IVCAP"
	URN_PREFIX = "ivcap"
)

const RELEASE_CHECK_URL = "https://github.com/ivcap-works/ivcap-cli/releases/latest"

// Max characters to limit name to
const MAX_NAME_COL_LEN = 30

var ACCESS_TOKEN_ENV = ENV_PREFIX + "_ACCESS_TOKEN"

// flags
var (
	contextName         string
	accessToken         string
	accessTokenF        string
	accessTokenProvided bool
	timeout             int
	debug               bool
)

var logger *log.Logger

type Config struct {
	Version       string    `yaml:"version"`
	ActiveContext string    `yaml:"active-context"`
	Contexts      []Context `yaml:"contexts"`
}

type Context struct {
	ApiVersion int    `yaml:"api-version"`
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	AccountID  string `yaml:"account-id"`
	ProviderID string `yaml:"provider-id"`
	Host       string `yaml:"host"` // set Host header if necessary

	// User Information
	AccountName     string `yaml:"account-name"`
	AccountNickName string `yaml:"account-nickname"`
	Email           string `yaml:"email"`

	// Cached Credentials
	AccessToken       string    `yaml:"access-token"`
	AccessTokenExpiry time.Time `yaml:"access-token-expiry"`
	RefreshToken      string    `yaml:"refresh-token"`
}

type AppError struct {
	msg string
}

func (e *AppError) Error() string { return fmt.Sprintf("ERROR: %s", e.msg) }

var rootCmd = &cobra.Command{
	Use:   "ivcap",
	Short: "A command line tool to interact with a IVCAP deployment",
	Long: `A command line tool to to more conveniently interact with the
API exposed by a specific IVCAP deployment.`,
}

func Execute(version string) {
	rootCmd.Version = version
	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
	if err := saveHistory(); err != nil {
		os.Exit(1)
	}
}

func CreateDoc() {
	initLogger()
	err := doc.GenMarkdownTree(rootCmd, "./doc")
	if err != nil {
		logger.Fatal("while creating markdown docs", log.Error(err))
	}
	header := &doc.GenManHeader{
		Title:   "IVCAP",
		Section: "3",
	}
	err = doc.GenManTree(rootCmd, header, "./doc")
	if err != nil {
		logger.Fatal("while creating man pages", log.Error(err))
	}
}

const DEFAULT_SERVICE_TIMEOUT_IN_SECONDS = 30

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "Context (deployment) to use")
	rootCmd.PersistentFlags().StringVar(&accessTokenF, "access-token", "",
		fmt.Sprintf("Access token to use for authentication with API server [%s]", ACCESS_TOKEN_ENV))
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", DEFAULT_SERVICE_TIMEOUT_IN_SECONDS, "Max. number of seconds to wait for completion")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Set logging level to DEBUG")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Set format for displaying output [json, yaml]")
	rootCmd.PersistentFlags().BoolVar(&silent, "silent", false, "Do not show any progress information")
	rootCmd.PersistentFlags().BoolVar(&noHistory, "no-history", false, "Do not store history")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	initLogger()
	// before proceeding, let's check for updates
	checkForUpdates(rootCmd.Version)
}

func initLogger() {
	cfg := log.NewDevelopmentConfig()
	// cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}

	logLevel := zapcore.ErrorLevel
	if debug {
		logLevel = zapcore.DebugLevel
	}
	cfg.Level = log.NewAtomicLevelAt(logLevel)
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	SetLogger(logger)
}

func CreateAdapter(requiresAuth bool) (adapter *adpt.Adapter) {
	return CreateAdapterWithTimeout(requiresAuth, timeout)
}

// Returns an HTTP adapter which will wait a max. of `timeoutSec` sec for a reply.
// It also pre-configures the adapter in the following way:
//
//   - If `requiresAuth` is set, each outgoing request includes an `Authorization` header
//     with a 'Bearer' token provided either via the `--access-token` flag,
//     the IVCAP_ACCESS_TOKEN environment, or the AccessToken from the ActiveContext.
//   - If the `path` parameter for any of the adapter calls is NOT a fully fledged URL,
//     the URL defined in ActiveContext is automatically prefixed.
//   - If the ActiveContext defines a `Host` parameter, it is also added as a
//     `Host` HTTP header.
func CreateAdapterWithTimeout(requiresAuth bool, timeoutSec int) (adapter *adpt.Adapter) {
	ctxt := GetActiveContext() // will always return with a context

	if requiresAuth {
		if accessToken == "" {
			accessToken = getAccessToken(true)
		}
		if accessToken == "" {
			cobra.CheckErr(
				fmt.Sprintf("Adapter requires auth token. Set with '--access-token' or env '%s'", ACCESS_TOKEN_ENV))
		}
	}

	url := ctxt.URL
	var headers *map[string]string
	if ctxt.Host != "" {
		headers = &(map[string]string{"Host": ctxt.Host})
	}
	logger.Debug("Adapter config", log.String("url", url))

	adp, err := NewAdapter(url, accessToken, timeoutSec, headers)
	if adp == nil || err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot create adapter for '%s' - %s", url, err))
	}
	return adp
}

// ****** ADAPTER ****

func NewAdapter(
	url string,
	accessToken string,
	timeoutSec int,
	headers *map[string]string,
) (*adpt.Adapter, error) {
	adapter := adpt.RestAdapter(adpt.ConnectionCtxt{
		URL: url, AccessToken: accessToken, TimeoutSec: timeoutSec, Headers: headers,
	})
	return &adapter, nil
}

func NewTimeoutContext() (ctxt context.Context, cancel context.CancelFunc) {
	to := time.Now().Add(time.Duration(timeout) * time.Second)
	ctxt, cancel = context.WithDeadline(context.Background(), to)
	return
}

func Logger() *log.Logger {
	return logger
}

func SetLogger(l *log.Logger) {
	logger = l
}

func safeString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func safeDate(s *string, humanizeOnly bool) string {
	if s != nil {
		t, err := time.Parse(time.RFC3339, *s)
		if err != nil {
			// fmt.Println("Error while parsing date :", err)
			return *s
		}
		h := humanize.Time(t)
		if humanizeOnly {
			return h
		} else {
			return fmt.Sprintf("%s (%s)", h, t.Local().Format(time.RFC822))
		}
	} else {
		return "???"
	}
}

func safeTruncString(in *string) (out string) {
	if in != nil {
		out = *in
	} else {
		out = "???"
	}
	if len(out) > MAX_NAME_COL_LEN {
		out = out[0:MAX_NAME_COL_LEN-3] + "..."
	}
	return
}

func safeBytes(n *int64) string {
	if n == nil {
		return "???"
	}
	if *n <= 0 {
		return "unknown"
	}
	// Safe to convert to uint64 here because we've checked for negative values
	// #nosec G115
	return humanize.Bytes(uint64(*n))
}

func payloadFromFile(fileName string, inputFormat string) (pyld adpt.Payload, err error) {
	isYaml := inputFormat == "yaml" || strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml")
	if fileName != "-" {
		pyld, err = adpt.LoadPayloadFromFile(fileName, isYaml)
	} else {
		pyld, err = adpt.LoadPayloadFromStdin(isYaml)
	}
	return
}

// ***** CHECK FOR NEWER VERSIONS

func checkForUpdates(currentVersion string) {
	path := makeConfigFilePath(VERSION_CHECK_FILE_NAME)
	if data, err := os.ReadFile(filepath.Clean(path)); err == nil {
		if lastCheck, err := time.Parse(time.RFC3339, string(data)); err == nil {
			d := time.Since(lastCheck)
			// fmt.Printf(".... since: %d < %d - %s\n", d, CHECK_VERSION_INTERVAL, path)
			if d < CHECK_VERSION_INTERVAL {
				// too soon
				return
			}
		} else {
			logger.Debug("cannot parse data in version check file", log.Error(err))
		}
	}

	// check latest versionpath string
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if resp, err := client.Head(RELEASE_CHECK_URL); err != nil {
		logger.Debug("checkForUpdates: while checking github", log.Error(err))
	} else {
		if loc, err := resp.Location(); err != nil {
			logger.Debug("checkForUpdates: while getting location", log.Error(err))
		} else {
			p := strings.Split(loc.Path, "/")
			latest := strings.TrimPrefix(p[len(p)-1], "v")
			current := strings.TrimPrefix(strings.Split(currentVersion, "|")[0], "v")
			if current != latest {
				fmt.Printf("\n>>>   A newer version 'v%s' is available. Please consider upgrading from 'v%s'", latest, current)
				fmt.Printf("\n>>>     It is available at %s", RELEASE_CHECK_URL)
				fmt.Printf("\n>>>     Or via 'brew upgrade ivcap'\n\n")
			}
		}
	}

	ts := time.Now().Format(time.RFC3339)
	if err := os.WriteFile(path, []byte(ts), fs.FileMode(0600)); err != nil {
		logger.Debug("cannot write version check timestamp", log.Error(err))
	}
}

func addNextPageRow(
	nextPage *string,
	pIn []table.Row,
) (pOut []table.Row) {
	if nextPage == nil {
		return pIn
	}
	u, err := url.Parse(*nextPage)
	if err == nil {
		page := u.Query().Get("page")
		pOut = pIn
		pOut = append(pOut, table.Row{"... " + MakeHistory(&page)})
	}
	return
}
