package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

var loginName string
var loginPassword string

var loginCmd = &cobra.Command{
	Use:   "login [flags] [user-name]",
	Short: "Authenticate with a specific deployment/context",
	Run:   loginF,
}

type LoginCmd struct {
	Name     string `json:"auth"`
	Password string `json:"password"`
}

type jwtInfo struct {
	Sub        string `json:"sub"`
	AccountID  string `json:"acc"`
	ProviderID string `json:"prv"`
	Expires    int    `json:"exp"`
}

func loginF(_ *cobra.Command, args []string) {
	ctxt := GetActiveContext()
	if len(args) > 0 {
		loginName = args[0]
	} else {
		if ctxt != nil && ctxt.LoginName != "" {
			loginName = ctxt.LoginName
		} else {
			cobra.CheckErr("Missing 'user-name'")
		}
	}
	// if loginPassword == "" {
	// 	loginPassword = os.Getenv("IVCAP_PASSWORD")
	// 	if loginPassword == "" {
	// 		loginPassword = util.GetPassword("password: ")
	// 	}
	// }
	cmd := &LoginCmd{Name: loginName, Password: loginPassword}
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Fatal("error marshalling body.", log.Error(err))
	}
	adapter := *CreateAdapter(false)
	adapter.ClearAuthorization() // remove any old authorization state
	if pyld, err := adapter.Post(context.Background(), "/1/sessions", bytes.NewReader(body), int64(len(body)), nil, logger); err != nil {
		cobra.CheckErr(fmt.Sprintf("login failed - %s", err))
	} else {
		token := string(pyld.AsBytes())
		var jwt jwtInfo
		jmid := strings.Split(token, ".")[1]
		ass, err := base64.RawStdEncoding.DecodeString(jmid)
		if err != nil {
			cobra.CheckErr(fmt.Sprintf("cannot decode JWT - %s", err))
			return
		}
		logger.Debug("jwt", log.ByteString("assertions", ass))
		if err = json.Unmarshal(ass, &jwt); err != nil {
			cobra.CheckErr(fmt.Sprintf("cannot parse JWT - %s", err))
			return
		}
		ctxt.Jwt = token
		ctxt.AccountID = jwt.AccountID
		ctxt.LoginName = loginName
		SetContext(ctxt, true)
		fmt.Println("Login succeeded")
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
	//loginCmd.Flags().StringVarP(&loginName, "name", "n", "", "Account name [IVCAP_NAME]")
}
