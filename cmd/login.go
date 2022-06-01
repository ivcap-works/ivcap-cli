package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/reinventingscience/ivcap-client/pkg/util"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

var loginName string
var loginPassword string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a specific deployment/context",
	// 	Long: `A longer description that spans multiple lines and likely contains examples
	// and usage of using your command. For example: ',
	Run: loginF,
}

type LoginCmd struct {
	Name     string `json:"auth"`
	Password string `json:"password"`
}

func loginF(_ *cobra.Command, _ []string) {
	if loginName == "" {
		cobra.CheckErr("Missing flag '--login-name'")
	}
	if loginPassword == "" {
		loginPassword = util.GetPassword("password: ")
	}
	cmd := &LoginCmd{Name: loginName, Password: loginPassword}
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Fatal("error marshalling body.", log.Error(err))
	}
	adapter := *CreateAdapter(false)
	adapter.ClearAuthorization() // remove any old authorization state
	if pyld, err := adapter.Post(context.Background(), "/1/sessions", bytes.NewReader(body), nil, logger); err != nil {
		cobra.CheckErr(fmt.Sprintf("login failed - %s", err))
	} else {
		token := string(pyld.AsBytes())
		ctxt := GetActiveContext()
		ctxt.Jwt = token
		ctxt.AccountID = loginName
		SetContext(ctxt, true)
		fmt.Println("Login succeeded")
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVarP(&loginName, "login-name", "n", "", "Account name")
	loginCmd.Flags().StringVarP(&loginPassword, "login-password", "p", "", "Account password")
}
