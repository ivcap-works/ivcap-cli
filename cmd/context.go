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
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(contextCmd)

	// LIST
	contextCmd.AddCommand(listContextCmd)

	// CREATE
	contextCmd.AddCommand(createContextCmd)
	// We don't really use them right, so better not confuse anyone
	// createContextCmd.Flags().StringVar(&accountID, "account-id", "", "The account ID to use. Will most likely be set on login")
	// createContextCmd.Flags().StringVar(&providerID, "provider-id", "", "The account ID to use. Will most likely be set on login")
	createContextCmd.Flags().StringVar(&hostName, "host-name", "", "optional host name if accessing API through SSH tunnel")
	createContextCmd.Flags().IntVar(&ctxtApiVersion, "version", 1, "define API version")

	// SET/USE
	contextCmd.AddCommand(useContextCmd)

	// READ/GET
	contextCmd.AddCommand(getContextCmd)
	getContextCmd.Flags().BoolVar(&refreshToken, "refresh-token", false, "if set, refresh access token if expired")
}

var (
	ctxtName       string
	ctxtApiVersion int
	hostName       string
	refreshToken   bool
)

// contextCmd represents the config command
var contextCmd = &cobra.Command{
	Use:     "context",
	Short:   "Manage and set access to various IVCAP deployments",
	Aliases: []string{"c"},
}

var createContextCmd = &cobra.Command{
	Use:   "create ctxtName https://ivcap.net",
	Short: "Create a new context",
	Args:  cobra.ExactArgs(2),
	//Aliases: []string{"create"},
	Run: func(_ *cobra.Command, args []string) {
		ctxtName = args[0]
		ctxtUrl := args[1]
		url, err := url.ParseRequestURI(ctxtUrl)
		if err != nil || url.Host == "" {
			cobra.CheckErr(fmt.Sprintf("url '%s' is not a valid URL", ctxtUrl))
		}

		ctxt := &Context{
			ApiVersion: ctxtApiVersion,
			Name:       ctxtName,
			URL:        ctxtUrl,
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
		config, _ := ReadConfigFile(true)
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
		param := "all"
		if len(args) == 1 {
			param = args[0]
		}
		context := GetActiveContext()
		if param == "name" {
			fmt.Println(context.Name)
		} else if param == "access-token" {
			if IsAuthorised() || refreshToken {
				fmt.Println(getAccessToken(true))
			} else {
				at := "NOT AUTHORISED\n"
				if silent {
					at = ""
				}
				fmt.Print(at)
			}
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
			t.AppendRow(table.Row{"Account ID", context.AccountID})
			if context.ProviderID != "" {
				t.AppendRow(table.Row{"Provider ID", context.ProviderID})
			}
			isAuth := "no"
			if IsAuthorised() {
				if accessTokenProvided {
					isAuth = fmt.Sprintf("unknown, token provided via '--access-token' flag or environment variable '%s'", ACCESS_TOKEN_ENV)
				} else {
					isAuth = fmt.Sprintf("yes, refreshing after %s", context.AccessTokenExpiry.Format(time.RFC822))
				}
			}
			t.AppendRow(table.Row{"Authorised", isAuth})
			if context.Host != "" {
				t.AppendRow(table.Row{"Host", context.Host})
			}

			t.Render()
		} else {
			cobra.CheckErr(fmt.Sprintf("unknown context parameter '%s'", param))
		}
	},
}
