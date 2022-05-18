package cmd

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	// "github.com/spf13/viper"

	adpt "github.com/reinventingscience/ivcap-client/pkg/adapter"

	"go.uber.org/zap"
	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const ENV_PREFIX = "IVCAP"

// Max characters to limit name to
const MAX_NAME_COL_LEN = 30

// flags
var (
	contextName string
	accountID   string
	debug       bool

	// common, but not global flags
	recordID string
	offset   int
	limit    int
	format   string
)

var logger *log.Logger

type Config struct {
	Version       string    `yaml:"version"`
	ActiveContext string    `yaml:"active-context""`
	Contexts      []Context `yaml:"contexts"`
}

type Context struct {
	ApiVersion int    `yaml:"api-version"`
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	AccountID  string `yaml:"account-id"`
	Jwt        string `yaml:"jwt"`
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
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) {
	// 	fmt.Printf("AUTHOR: %s - %s\n", author, viper.GetString("author"))
	// },
}

func Execute(version string) {
	rootCmd.Version = version
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// rootCmd.PersistentFlags().StringVar(&url, "url", "", "url to the IVCAP deployment (e.g. https://api.green-cirrus.com)")
	// rootCmd.PersistentFlags().StringVar(&jwt, "jwt", "", "Authentication token")
	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "Context (deployment) to use")
	rootCmd.PersistentFlags().StringVar(&accountID, "account-id", "", "Account ID to use with requests. Most likely defined in context")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Set logging level to DEBUG")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	//var cfg log.Config
	cfg := zap.NewDevelopmentConfig()
	// cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}

	logLevel := zapcore.InfoLevel
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

func GetAccountID() string {
	if accountID == "" {
		accountID = os.Getenv(ENV_PREFIX + "_ACCOUNT_ID")
		if accountID == "" {
			if ctxt := GetActiveContext(); ctxt != nil {
				accountID = ctxt.AccountID
			}
		}
	}
	if accountID == "" {
		cobra.CheckErr("account ID is not set. Use the --account-id flag or environment IVCAP_ACCOUNT_ID")
	}
	return accountID
}

func CreateAdapter(requiresAuth bool) (adapter *adpt.Adapter) {
	if contextName == "" {
		contextName = os.Getenv(ENV_PREFIX + "_CONTEXT")
	}

	url := os.Getenv(ENV_PREFIX + "_URL")
	jwt := os.Getenv(ENV_PREFIX + "_JWT")

	if url == "" || jwt == "" {
		// check config file
		ctxt := GetActiveContext()
		if ctxt == nil {
			cobra.CheckErr("cannot find a respective context")
		}
		if url == "" {
			url = ctxt.URL
		}
		if jwt == "" {
			jwt = ctxt.Jwt
		}
	}

	logger.Debug("Adapter config", log.String("url", url), log.String("jwt", jwt))
	if url == "" {
		cobra.CheckErr("required context 'url' not set")
	}
	adp, err := NewAdapter(url, jwt)
	if adp == nil || err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot create adapter for '%s' - %s", url, err))
	}
	return adp
}

func GetActiveContext() (ctxt *Context) {
	config, configFile := ReadConfigFile(false)
	if config != nil {
		if contextName == "" {
			contextName = config.ActiveContext
		}
		if contextName != "" {
			for _, d := range config.Contexts {
				if d.Name == contextName {
					ctxt = &d
					break
				}
			}
			if ctxt == nil {
				cobra.CheckErr(fmt.Sprintf("unknown context '%s' in config '%s'", contextName, configFile))
			}
		}
	}
	return ctxt
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

// func CreateContext(ctxt *Context) {
// 	config, _ := ReadConfigFile(true)
// 	// if err != nil {
// 	// 	if _, ok := err.(*os.PathError); !ok {
// 	// 		// if perr := err.(fs.PathError); perr != nil {
// 	// 		fmt.Printf("ERROR %v", err)
// 	// 	}
// 	// }
// 	// if config == nil {
// 	// 	config = &Config{
// 	// 		Version: "v1",
// 	// 	}
// 	// }
// 	cxa := config.Contexts
// 	for i, c := range cxa {
// 		if c.Name == ctxt.Name {
// 			config.Contexts[i] = *ctxt
// 			WriteConfigFile(config)
// 			return
// 		}
// 	}
// 	// First context, make it the active/default one as well
// 	config.Contexts = append(config.Contexts, *ctxt)
// 	config.ActiveContext = ctxt.Name
// 	WriteConfigFile(config)
// }

func ReadConfigFile(createIfNoConfig bool) (config *Config, configFile string) {
	configFile = os.Getenv("HOME") + "/.ivcap"
	var data []byte
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		if _, ok := err.(*os.PathError); createIfNoConfig && ok {
			config = &Config{
				Version: "v1",
			}
			return
		}
		cobra.CheckErr(fmt.Sprintf("problems while reading config file %s - %v", configFile, err))
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
	configFile := os.Getenv("HOME") + "/.ivcap"
	if err = ioutil.WriteFile(configFile, b, fs.FileMode(0644)); err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot write to config file %s - %v", configFile, err))
	}
}

func NewAdapter(url string, jwtToken string) (*adpt.Adapter, error) {
	adapter := adpt.RestAdapter(adpt.ConnectionCtxt{
		URL: url, JwtToken: jwtToken,
	})
	return &adapter, nil
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
