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
	"errors"
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
	"gopkg.in/yaml.v2"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const ENV_PREFIX = "IVCAP"
const URN_PREFIX = "ivcap"

const RELEASE_CHECK_URL = "https://github.com/ivcap-works/ivcap-cli/releases/latest"

// Max characters to limit name to
const MAX_NAME_COL_LEN = 30

// Names for config dir and file - stored in the os.UserConfigDir() directory
const CONFIG_FILE_DIR = "ivcap-cli"
const CONFIG_FILE_NAME = "config.yaml"
const HISTORY_FILE_NAME = "history.yaml"
const VERSION_CHECK_FILE_NAME = "vcheck.txt"
const CHECK_VERSION_INTERVAL = time.Duration(24 * time.Hour)

var ACCESS_TOKEN_ENV = ENV_PREFIX + "_ACCESS_TOKEN"

const DEF_LIMIT = 10

// flags
var (
	contextName         string
	accessToken         string
	accessTokenF        string
	accessTokenProvided bool
	timeout             int
	debug               bool

	// common, but not global flags
	recordID     string
	offset       int
	limit        int
	outputFormat string
	silent       bool

	schemaURN        string
	schemaPrefix     string
	entityURN        string
	aspectJsonFilter string
	aspectFilter     string
	atTime           string
	page             string
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
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// var cfg log.Config
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
	// before proceeding, let's check for updates
	checkForUpdates(rootCmd.Version)
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

func GetActiveContext() (ctxt *Context) {
	return GetContext(contextName, true) // choose active context
}

func GetContext(name string, defaultToActiveContext bool) (ctxt *Context) {
	var err error
	ctxt, err = GetContextWithError(name, defaultToActiveContext)
	if err != nil {
		cobra.CheckErr(err)
	}
	return
}

func GetContextWithError(name string, defaultToActiveContext bool) (ctxt *Context, err error) {
	config, configFile := ReadConfigFile(true)
	// config should never be nil
	if name == "" && defaultToActiveContext {
		name = config.ActiveContext
	}
	if name == "" {
		// no context or active context is found
		return nil, errors.New("cannot find suitable context. Use '--context' or set default via 'context' command")
	}

	for idx, d := range config.Contexts {
		if d.Name == name {
			return &config.Contexts[idx], nil // golang loop reuse same var, don't use "&d"
		}
	}

	if ctxt == nil {
		return nil, fmt.Errorf("unknown context '%s' in config '%s'", name, configFile)
	}
	return
}

func SetContext(ctxt *Context, failIfNotExist bool) {
	config, _ := ReadConfigFile(true)
	cxa := config.Contexts
	for i, c := range cxa {
		if c.Name == ctxt.Name {
			config.Contexts[i] = *ctxt
			WriteConfigFile(config)
			return
		}
	}
	if failIfNotExist {
		cobra.CheckErr(fmt.Sprintf("attempting to set/update non existing context '%s'", ctxt.Name))
	} else {
		config.Contexts = append(config.Contexts, *ctxt)
		if len(config.Contexts) == 1 {
			// First context, make it the active/default one as well
			config.ActiveContext = ctxt.Name
		}
		WriteConfigFile(config)
	}
}

func ReadConfigFile(createIfNoConfig bool) (config *Config, configFile string) {
	configFile = GetConfigFilePath()
	var data []byte
	data, err := os.ReadFile(filepath.Clean(configFile))
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			if createIfNoConfig {
				config = &Config{
					Version: "v1",
				}
				return
			} else {
				cobra.CheckErr("Config file does not exist. Please create the config file with the context command.")
			}
		} else {
			cobra.CheckErr(fmt.Sprintf("Cannot read config file %s - %v", configFile, err))
		}
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		cobra.CheckErr(fmt.Sprintf("problems parsing config file %s - %v", configFile, err))
		return
	}
	config = &cfg
	return
}

func WriteConfigFile(config *Config) {
	b, err := yaml.Marshal(config)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot marshall content of config file - %v", err))
		return
	}

	configFile := GetConfigFilePath()

	if err = os.WriteFile(configFile, b, fs.FileMode(0600)); err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot write to config file %s - %v", configFile, err))
	}
}

func GetConfigDir(createIfNoExist bool) (configDir string) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot find the user configuration directory - %v", err))
		return
	}
	configDir = userConfigDir + string(os.PathSeparator) + CONFIG_FILE_DIR
	// Create it if it doesn't exist
	if createIfNoExist {
		err = os.MkdirAll(configDir, 0750)
		if err != nil && !os.IsExist(err) {
			cobra.CheckErr(fmt.Sprintf("Could not create configuration directory %s - %v", configDir, err))
			return
		}
	}
	return
}

func GetConfigFilePath() (path string) {
	path = makeConfigFilePath(CONFIG_FILE_NAME)
	return
}

func makeConfigFilePath(fileName string) (path string) {
	configDir := GetConfigDir(true) // Create the configuration directory if it doesn't exist
	path = configDir + string(os.PathSeparator) + fileName
	return
}

// ****** HISTORY ****

var history map[string]string

func MakeHistory(urn *string) string {
	if urn == nil {
		return "???"
	}
	if history == nil {
		history = make(map[string]string)
	}
	token := fmt.Sprintf("@%d", len(history)+1)
	history[token] = *urn
	return token
}

// Check if argument is an IVCAP urn and if it
// is, turn it into a history.
func MakeMaybeHistory(sp *string) string {
	if sp == nil {
		return "???"
	}
	// HACK: Should go away in future IVCAP Core version
	if strings.HasPrefix(*sp, "http://artifact.local/") {
		u := *sp
		u = u[len("http://artifact.local/"):]
		sp = &u
	}
	// We assume, all IVCAP urns follow the pattern 'urn:ivcap:_service_:...
	if !strings.HasPrefix(*sp, "urn:ivcap:") {
		// no it's not
		return *sp
	}

	if history == nil {
		history = make(map[string]string)
	}
	token := fmt.Sprintf("@%d", len(history)+1)
	history[token] = *sp
	return fmt.Sprintf("%s (%s)", token, *sp)
}

func GetHistory(token string) (value string) {
	if !strings.HasPrefix(token, "@") {
		return token
	}
	var vp *string
	path := getHistoryFilePath()
	var data []byte
	data, err := os.ReadFile(filepath.Clean(path))
	var hm map[string]string
	if err == nil {
		if err := yaml.Unmarshal(data, &hm); err != nil {
			cobra.CheckErr(fmt.Sprintf("problems parsing history file %s - %v", path, err))
			return
		}
		if val, ok := hm[token]; ok {
			vp = &val
		}
	} else {
		// fail "normally" if file doesn't exist
		if _, ok := err.(*os.PathError); !ok {
			cobra.CheckErr("Error reading history file. Use full names instead.")
			return
		}
	}
	if vp == nil {
		cobra.CheckErr(fmt.Sprintf("Unknown history '%s'.", token))
		return
	}
	return *vp
}

func saveHistory() (err error) {
	if history == nil {
		return
	}

	b, err := yaml.Marshal(history)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot marshall history - %v", err))
		return
	}

	path := makeConfigFilePath(HISTORY_FILE_NAME)

	if err = os.WriteFile(path, b, fs.FileMode(0600)); err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot write history to file %s - %v", path, err))
	}
	return
}

func getHistoryFilePath() (path string) {
	return makeConfigFilePath(HISTORY_FILE_NAME)
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
	} else {
		return ""
	}
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
	if n != nil {
		if *n <= 0 {
			return "unknown"
		}
		return humanize.Bytes(uint64(*n))
	} else {
		return "???"
	}
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
