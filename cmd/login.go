package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

var loginName string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a specific deployment/context",
	// 	Long: `A longer description that spans multiple lines and likely contains examples
	// and usage of using your command. For example: ',
	Run: loginF,
}

type LoginCmd struct {
	Name string `json:"auth"`
}

func loginF(_ *cobra.Command, _ []string) {
	if loginName == "" {
		cobra.CheckErr("Missing flag '--name'")
	}
	cmd := &LoginCmd{loginName}
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Fatal("error marshalling body.", log.Error(err))
	}
	adapter := *CreateAdapter(false)
	adapter.ClearAuthorization() // remove any old authorization state
	if pyld, err := adapter.Post(context.Background(), "/1/sessions", bytes.NewReader(body), logger); err != nil {
		cobra.CheckErr(fmt.Sprintf("login failed - %s", err))
	} else {
		token := string(pyld.AsBytes())
		ctxt := GetActiveContext()
		ctxt.Jwt = token
		ctxt.AccountID = loginName
		SetContext(ctxt, true)
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVarP(&loginName, "name", "n", "", "Account name")
}
