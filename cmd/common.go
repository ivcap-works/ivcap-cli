package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
)

// Names for config dir and file - stored in the os.UserConfigDir() directory
const CONFIG_FILE_DIR = "ivcap-cli"
const CONFIG_FILE_NAME = "config.yaml"
const HISTORY_FILE_NAME = "history.yaml"
const VERSION_CHECK_FILE_NAME = "vcheck.txt"
const CHECK_VERSION_INTERVAL = time.Duration(24 * time.Hour)

const DEF_LIMIT = 10

var (
	// common, but not global flags
	recordID string

	limit     int
	page      string
	filter    string
	orderBy   string
	orderDesc bool
	atTime    string

	fileName     string
	outputFormat string
	silent       bool
	noHistory    bool

	schemaURN    string
	schemaPrefix string
	entityURN    string
)

// ****** FLAGS ****

type Flag int64

const (
	Name Flag = iota
	Schema
	Entity
	InputFormat
	Policy
	Account
	AtTime
	File
)

func addListFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.IntVar(&limit, "limit", DEF_LIMIT, "max number of records to be returned")
	fs.StringVarP(&page, "page", "p", "", "page cursor")
	fs.StringVar(&filter, "filter", "", "filter list (e.g. \"name~=Fred\")")
	fs.StringVar(&orderBy, "order-by", "", "feature to order list by (e.g. \"created-at,status\")")
	fs.BoolVar(&orderDesc, "order-desc", false, "if set, order in descending order")
	fs.StringVar(&atTime, "at-time", "", "query state at this time in the past")
}

func addFlags(cmd *cobra.Command, names []Flag) {
	for _, n := range names {
		switch n {
		case Name:
			addNameFlag(cmd)
		case Schema:
			addSchemaFlag(cmd)
		case Entity:
			addEntityFlag(cmd)
		case InputFormat:
			addInputFormatFlag(cmd)
		case Policy:
			addPolicyFlag(cmd)
		case Account:
			addAccountFlag(cmd)
		case AtTime:
			addAtTimeFlag(cmd)
		case File:
			addFileFlag(cmd, "")
		default:
			panic(fmt.Sprintf("Missing implementation for flag '%v'", n))
		}
	}
}

func addNameFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVarP(&name, "name", "n", "", "Human friendly name")
}

func addPolicyFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVarP(&policy, "policy", "p", "", "Policy controlling access")
}

func addAccountFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&accountID, "account-id", "", "override the account ID to use for this request")
}

func addFileFlag(cmd *cobra.Command, kind string) {
	fs := cmd.Flags()
	usage := kind
	if usage == "" {
		usage = "Path to input file"
	}
	fs.StringVarP(&fileName, "file", "f", "", usage)
}

func addInputFormatFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&inputFormat, "format", "json", "Format of input file [json, yaml]")
}

func addSchemaFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVarP(&schemaURN, "schema", "s", "", "URN/UUID of schema")
}

func addEntityFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVarP(&entityURN, "entity", "e", "", "URN/UUID of entity")
}

func addAtTimeFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&atTime, "at-time", "", "query state at this time in the past")
}

func createListRequest() (req *sdk.ListRequest) {
	req = &sdk.ListRequest{Limit: DEF_LIMIT}
	if limit > 0 {
		req.Limit = limit
	}
	if page != "" {
		p := GetHistory(page)
		req.Page = &p
	}
	if filter != "" {
		req.Filter = &filter
	}
	if orderBy != "" {
		req.OrderBy = &orderBy
	}
	req.OrderDesc = orderDesc
	if atTime != "" {
		t, err := dateparse.ParseLocal(atTime)
		if err != nil {
			cobra.CheckErr(fmt.Sprintf("Can't parse '%s' into a date - %s", atTime, err))
		}
		req.AtTime = &t
	}
	return
}

// ****** HISTORY ****

var history map[string]string

func MakeHistory(urn *string) string {
	if urn == nil {
		return "???"
	}
	if noHistory {
		return *urn
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
	if noHistory {
		return *sp
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

// ****** CONTEXT ****

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

// ****** CONFIG FILE ****

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
