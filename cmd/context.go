package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var ctxtName string
var ctxtUrl string
var ctxtApiVersion int

var printJWT bool
var printAccountID bool
var printProviderID bool
var printURL bool

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:     "context",
	Short:   "Manage and set access to various IVCAP deployments",
	Aliases: []string{"c"},
}

var setContextCmd = &cobra.Command{
	Use:   "create ctxtName --url https://ivcap.net",
	Short: "Create a new context",
	//Aliases: []string{"create"},
	Run: func(_ *cobra.Command, args []string) {
		if ctxtName == "" {
			if len(args) > 0 {
				ctxtName = args[0]
			} else {
				cobra.CheckErr("Missing 'name' argument or '--name' flag")
			}
		}
		if ctxtUrl == "" {
			cobra.CheckErr("Missing '--url' flag")
		}
		url, err := url.ParseRequestURI(ctxtUrl)
		if err != nil || url.Host == "" {
			cobra.CheckErr(fmt.Sprintf("url '%s' is not a valid URL", ctxtUrl))
		}

		ctxt := &Context{
			ApiVersion: ctxtApiVersion,
			Name:       ctxtName,
			URL:        ctxtUrl,
		}
		SetContext(ctxt, false)
		fmt.Printf("Context '%s' created.\n", ctxtName)
	},
}

var listContextCmd = &cobra.Command{
	Use:   "list",
	Short: "List all context",
	//Aliases: []string{"get-context", "list"},
	Run: func(_ *cobra.Command, _ []string) {
		config, _ := ReadConfigFile(false)
		if config != nil {
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.AppendHeader(table.Row{"Current", "Name", "AccountID", "URL"})
			active := config.ActiveContext
			for _, c := range config.Contexts {
				current := ""
				if active == c.Name {
					current = "*"
				}
				t.AppendRow(table.Row{current, c.Name, c.AccountID, c.URL})
			}
			t.Render()
		}
	},
}

var useContextCmd = &cobra.Command{
	Use:     "set name",
	Short:   "Set the current context in the config file",
	Aliases: []string{"use"},
	Run: func(_ *cobra.Command, args []string) {
		if len(args) < 1 {
			cobra.CheckErr("Missing 'name' arg")
		}
		ctxtName = args[0]
		config, _ := ReadConfigFile(false)
		ctxtExists := false
		for _, c := range config.Contexts {
			if c.Name == ctxtName {
				ctxtExists = true
				break
			}
		}
		if ctxtExists {
			config.ActiveContext = ctxtName
			WriteConfigFile(config)
			fmt.Printf("Switched to context '%s'.\n", ctxtName)
		} else {
			cobra.CheckErr(fmt.Sprintf("context '%s' is not defined", ctxtName))
		}
	},
}

var getContextCmd = &cobra.Command{
	Use:     "get",
	Short:   "Display the current context",
	Aliases: []string{"current", "show"},
	Run: func(_ *cobra.Command, _ []string) {
		context := GetActiveContext()
		if printJWT {
			fmt.Println(context.Jwt)
		} else if printAccountID {
			fmt.Println(context.AccountID)
		} else if printProviderID {
			fmt.Println(context.ProviderID)
		} else if printURL {
			fmt.Println(context.URL)
		} else {
			fmt.Println(context.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(listContextCmd)

	configCmd.AddCommand(setContextCmd)
	setContextCmd.Flags().StringVar(&ctxtUrl, "url", "", "url to the IVCAP deployment (e.g. https://api.green-cirrus.com)")
	setContextCmd.Flags().IntVar(&ctxtApiVersion, "version", 1, "define API version")

	configCmd.AddCommand(useContextCmd)

	configCmd.AddCommand(getContextCmd)
	getContextCmd.Flags().BoolVar(&printJWT, "jwt", false, "Print the currently active JWT token")
	getContextCmd.Flags().BoolVar(&printAccountID, "account-id", false, "Print the currently active account ID")
	getContextCmd.Flags().BoolVar(&printProviderID, "provider-id", false, "Print the currently active provider ID")
	getContextCmd.Flags().BoolVar(&printURL, "url", false, "Print the URL of the currently active deployment")
}
