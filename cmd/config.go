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

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

var setContextCmd = &cobra.Command{
	Use:   "create-context ctxtName --url https://ivcap.net",
	Short: "Create a new context",
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
	Use:   "get-contexts",
	Short: "List all context",
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
	Use:   "use-context name",
	Short: "Set the current-context in the config file",
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

var currentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Display the current-context",
	Run: func(_ *cobra.Command, _ []string) {
		config, _ := ReadConfigFile(false)
		fmt.Println(config.ActiveContext)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(listContextCmd)

	configCmd.AddCommand(setContextCmd)
	setContextCmd.Flags().StringVar(&ctxtUrl, "url", "", "url to the IVCAP deployment (e.g. https://api.green-cirrus.com)")
	setContextCmd.Flags().IntVar(&ctxtApiVersion, "version", 1, "define API version")

	configCmd.AddCommand(useContextCmd)

	configCmd.AddCommand(currentContextCmd)

}
