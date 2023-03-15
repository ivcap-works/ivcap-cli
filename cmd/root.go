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
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	adpt "github.com/reinventingscience/ivcap-client/pkg/adapter"

	"go.uber.org/zap"
	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const ENV_PREFIX = "IVCAP"
const URN_PREFIX = "ivcap"

// Max characters to limit name to
const MAX_NAME_COL_LEN = 30

// Names for config dir and file - stored in the os.UserConfigDir() directory
const CONFIG_FILE_DIR = "ivcap-cli"
const CONFIG_FILE_NAME = "config.yaml"

var ACCESS_TOKEN_ENV = ENV_PREFIX + "_ACCESS_TOKEN"

// flags
var (
	contextName string
	accessToken string
	timeout     int
	debug       bool

	// common, but not global flags
	recordID     string
	offset       int
	limit        int
	outputFormat string
	silent       bool
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
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "Context (deployment) to use")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "",
		fmt.Sprintf("Access token to use for authentication with API server [%s]", ACCESS_TOKEN_ENV))
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 10, "Max. number of seconds to wait for completion")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Set logging level to DEBUG")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Set format for displaying output [json, yaml]")
	rootCmd.PersistentFlags().BoolVar(&silent, "silent", false, "Do not show any progress information")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	//var cfg log.Config
	cfg := zap.NewDevelopmentConfig()
	// cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}

	logLevel := zapcore.ErrorLevel
	if debug {
		logLevel = zapcore.DebugLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(logLevel)
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	SetLogger(logger)
}

func CreateAdapter(requiresAuth bool) (adapter *adpt.Adapter) {
	return CreateAdapterWithTimeout(requiresAuth, timeout)
}

func CreateAdapterWithTimeout(requiresAuth bool, timeoutSec int) (adapter *adpt.Adapter) {
	ctxt := GetActiveContext() // will always return with a context

	if requiresAuth {
		if accessToken == "" {
			accessToken = os.Getenv(ACCESS_TOKEN_ENV)
		}
		if accessToken == "" {
			// If the user hasn't provided an access token as an environmental variable
			// we'll assume the user has logged in previously. We call refreshAccessToken
			// here, so that we'll check the current access token, and if it has expired,
			// we'll use the refresh token to get ourselves a new one. If the refresh
			// token has expired, we'll prompt the user to login again.
			accessToken = getAccessToken()
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

	config, configFile := ReadConfigFile(true)
	// config should never be nil
	if name == "" && defaultToActiveContext {
		name = config.ActiveContext
	}
	if name == "" {
		// no context or active context is found
		cobra.CheckErr("Cannot find suitable context. Use '--context' or set default via 'context' command")
		return
	}

	for _, d := range config.Contexts {
		if d.Name == name {
			ctxt = &d
			return
		}
	}

	if ctxt == nil {
		cobra.CheckErr(fmt.Sprintf("unknown context '%s' in config '%s'", name, configFile))
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
	data, err := ioutil.ReadFile(configFile)
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

	if err = ioutil.WriteFile(configFile, b, fs.FileMode(0600)); err != nil {
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
		err = os.MkdirAll(configDir, 0755)
		if err != nil && !os.IsExist(err) {
			cobra.CheckErr(fmt.Sprintf("Could not create configuration directory %s - %v", configDir, err))
			return
		}
	}
	return
}

func GetConfigFilePath() (configFile string) {
	configDir := GetConfigDir(true) // Create the configuration directory if it doesn't exist
	configFile = configDir + string(os.PathSeparator) + CONFIG_FILE_NAME
	return
}

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

func NewTimeoutContext() (ctxt context.Context) {
	to := time.Now().Add(time.Duration(timeout) * time.Second)
	ctxt, _ = context.WithDeadline(context.Background(), to)
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
		return "???"
	}
}

func safeDate(s *string) string {
	if s != nil {
		t, err := time.Parse(time.RFC3339, *s)
		if err != nil {
			// fmt.Println("Error while parsing date :", err)
			return *s
		}
		return t.Local().Format(time.RFC822)
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

func safeNumber(n *int64) string {
	if n != nil {
		if *n <= 0 {
			return "unknown"
		}
		return strconv.Itoa(int(*n))
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
