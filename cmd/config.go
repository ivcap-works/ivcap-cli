package cmd

import (
	"fmt"
	"net/url"

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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("config called")
	},
}

var setContextCmd = &cobra.Command{
	Use:   "create-context",
	Short: "Create a new context",
	// 	Long: `A longer description that spans multiple lines and likely contains examples
	// and usage of using your command. For example: ',
	Run: func(_ *cobra.Command, _ []string) {
		if ctxtName == "" {
			cobra.CheckErr("Missing '--name' flag")
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
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(setContextCmd)
	setContextCmd.Flags().StringVar(&ctxtName, "name", "", "Name of context")
	setContextCmd.Flags().StringVar(&ctxtUrl, "url", "", "url to the IVCAP deployment (e.g. https://api.green-cirrus.com)")
	setContextCmd.Flags().IntVar(&ctxtApiVersion, "version", 1, "define API version [1]")
}
