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

var (
	accountID  string
	providerID string
	hostName   string
)

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
			AccountID:  accountID,
			ProviderID: providerID,
			LoginName:  loginName,
			Host:       hostName,
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
	Use:     "get [all|name|account-id|provider-id|url|access-token]",
	Short:   "Display the current context",
	Aliases: []string{"current", "show"},
	Run: func(_ *cobra.Command, args []string) {
		param := "name"
		if len(args) == 1 {
			param = args[0]
		}
		context := GetActiveContext()
		if param == "name" {
			fmt.Println(context.Name)
		} else if param == "access-token" {
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.AppendRow(table.Row{"Access Token", context.AccessToken})
			t.AppendRow(table.Row{"Token Expiry", context.AccessTokenExpiry})
			t.Render()
		} else if param == "account-id" {
			fmt.Println(context.AccountID)
		} else if param == "provider-id" {
			fmt.Println(context.ProviderID)
		} else if param == "url" {
			fmt.Println(context.URL)
		} else if param == "all" {
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.AppendRow(table.Row{"Name", context.Name})
			t.AppendRow(table.Row{"URL", context.URL})
			t.AppendRow(table.Row{"Login Name", context.LoginName})
			t.AppendRow(table.Row{"Account ID", context.AccountID})
			if context.ProviderID != "" {
				t.AppendRow(table.Row{"Provider ID", context.ProviderID})
			}
			if context.Host != "" {
				t.AppendRow(table.Row{"Host", context.Host})
			}
			t.Render()
		} else {
			cobra.CheckErr(fmt.Sprintf("unknown context parameter '%s'", param))
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(listContextCmd)

	configCmd.AddCommand(setContextCmd)
	setContextCmd.Flags().StringVar(&ctxtUrl, "url", "", "The url to the IVCAP deployment (e.g. https://api.green-cirrus.com)")
	setContextCmd.Flags().StringVar(&loginName, "login-name", "", "Name for authentication. May not be required depending on Auth mechanism")
	setContextCmd.Flags().StringVar(&accountID, "account-id", "", "The account ID to use. Will most likely be set on login")
	setContextCmd.Flags().StringVar(&providerID, "provider-id", "", "The account ID to use. Will most likely be set on login")
	setContextCmd.Flags().StringVar(&hostName, "host-name", "", "optional host name if accessing API through SSH tunnel")
	setContextCmd.Flags().IntVar(&ctxtApiVersion, "version", 1, "define API version")

	configCmd.AddCommand(useContextCmd)

	configCmd.AddCommand(getContextCmd)
}
