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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
	adpt "github.com/reinventingscience/ivcap-client/pkg/adapter"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
	"golang.org/x/oauth2"
	yaml "gopkg.in/yaml.v3"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a specific deployment/context",
	Run:   login,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove authentication tokens from specific deployment/context",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		ctxt := GetActiveContext()
		ctxt.AccessToken = ""
		ctxt.AccessTokenExpiry = time.Time{}
		ctxt.RefreshToken = ""
		SetContext(ctxt, true)
		return
	},
}

type CaddyFaultResponse struct {
	Name      string
	Id        string
	Message   string
	Temporary bool
	Timeout   bool
	Fault     bool
}

type AuthInfo struct {
	Version      int              `yaml:"version"`
	ProviderList AuthProviderInfo `yaml:"auth"`
}

type AuthProviderInfo struct {
	DefaultProviderId string                  `yaml:"default-provider-id"`
	AuthProviders     map[string]AuthProvider `yaml:"providers"`
}

type AuthProvider struct {
	ID        string `yaml:"id,omitempty"`
	LoginURL  string `yaml:"login-url"`
	TokenURL  string `yaml:"token-url"`
	CodeURL   string `yaml:"code-url"`
	JwksURL   string `yaml:"jwks-url"`
	ClientID  string `yaml:"client-id"`
	audience  string
	scopes    string
	grantType string
}

type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURL         string `json:"verification_uri"`
	VerificationURLComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

type CustomIdClaims struct {
	Name          string   `json:"name,omitempty"`
	Nickname      string   `json:"nickname,omitempty"`
	Email         string   `json:"email,omitempty"`
	EmailVerified bool     `json:"email_verified,omitempty"`
	Avatar        string   `json:"picture,omitempty"`
	AccountID     string   `json:"acc"`
	ProviderID    string   `json:"ivcap/claims/provider,omitempty"`
	GroupIDs      []string `json:"ivcap/claims/groupIds,omitempty"`
	jwt.RegisteredClaims
}

type deviceTokenResponse struct {
	*oauth2.Token
	IDToken     string `json:"id_token,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
	ErrorString string `json:"error,omitempty"`
}

// First check environment variables and command line flags for provided
// tokens and immedaitely return them if available. Then check the 'ActiveContext'
// for a token and if `refreshIfExpired` is set, ckeck if token is expired and
// if it is, request a new one from the identitiy provider.
func getAccessToken(refreshIfExpired bool) (accessToken string) {
	if accessTokenF != "" {
		accessTokenProvided = true
		return accessTokenF
	}
	if accessToken = os.Getenv(ACCESS_TOKEN_ENV); accessToken != "" {
		accessTokenProvided = true
		return
	}

	// If the user hasn't provided an access token as an environmental variable
	// we'll assume the user has logged in previously. We call refreshAccessToken
	// here, so that we'll check the current access token, and if it has expired,
	// we'll use the refresh token to get ourselves a new one. If the refresh
	// token has expired, we'll prompt the user to login again.
	ctxt := GetActiveContext()
	accessTokenExpiry := ctxt.AccessTokenExpiry
	if time.Now().After(accessTokenExpiry) {
		if !refreshIfExpired {
			return ""
		}
		if ctxt.RefreshToken == "" {
			// We don't have a refresh token for this context, so we fail early
			cobra.CheckErr("Could not login - invalid credentials. Please use the login command to refresh your credentials")
		}

		// Access token has expired, we have to refresh it
		authProvider := getLoginInformation(ctxt)
		authProvider.grantType = "refresh_token"

		if (authProvider.TokenURL != "") && (authProvider.ClientID != "") {
			params := url.Values{
				"refresh_token": {ctxt.RefreshToken},
			}
			tokenResponse := getTokenResponse(authProvider, params, ctxt, false)
			if tokenResponse.ErrorString != "" {
				logger.Warn("tokenResponse", log.String("error", tokenResponse.ErrorString))
				cobra.CheckErr("oauth: Unexpected error from authentication provider")
			}

			ctxt.AccessToken = tokenResponse.AccessToken
			if tokenResponse.RefreshToken != "" {
				ctxt.RefreshToken = tokenResponse.RefreshToken
			}
			// Add a 10 second buffer to expiry to account for differences in clock time between client
			// server and message transport time (oauth2 library does the same thing)
			ctxt.AccessTokenExpiry = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn-10))

			// We also get an updated ID token, let's make sure we have the latest info
			ParseIDToken(&tokenResponse, ctxt, authProvider.JwksURL)
			SetContext(ctxt, true)
			logger.Info("Successfully acquired new access token.", log.String("expires", ctxt.AccessTokenExpiry.Format(time.RFC822)))
		} // Access token has not expired, let's just use it
	}

	return ctxt.AccessToken
}

func IsAuthorised() bool {
	return getAccessToken(false) != ""
}

func getTokenResponse(authProvider *AuthProvider, params url.Values, ctxt *Context, allowStatusForbidden bool) (tokenResponse deviceTokenResponse) {
	adapter := CreateAdapter(false)
	params.Set("grant_type", authProvider.grantType)
	params.Set("client_id", authProvider.ClientID)

	var pyld adpt.Payload
	var err error
	pyld, err = (*adapter).PostForm(NewTimeoutContext(), authProvider.TokenURL, params, nil, logger)
	if err != nil {
		if apiErr, ok := err.(*adpt.ApiError); ok && allowStatusForbidden {
			if apiErr.StatusCode == http.StatusForbidden {
				pyld = apiErr.Payload
			} else {
				cobra.CheckErr(fmt.Sprintf("Cannot obtain OAuth Token - %s", err))
			}
		} else {
			cobra.CheckErr(fmt.Sprintf("Cannot obtain OAuth Token - %s", err))
			return // never reached
		}
	}

	if err = pyld.AsType(&tokenResponse); err != nil {
		logger.Error("while parsing 'deviceTokenResponse'", log.String("pyld", string(pyld.AsBytes())))
		cobra.CheckErr("oauth: Cannot decode token response")
		return
	}

	switch tokenResponse.ErrorString {
	case "expired_token":
		cobra.CheckErr("The login process was not completed in time - please login again")
	case "access_denied":
		cobra.CheckErr("Could not login - access was denied")
	case "invalid_grant":
		cobra.CheckErr("Could not login - expired credentials. Please use the login command to refresh your credentials")
	}
	return
}

func getLoginInformation(ctxt *Context) (authProvider *AuthProvider) {
	adpt := CreateAdapter(false)
	pyld, err := (*adpt).Get(NewTimeoutContext(), "/1/authinfo.yaml", logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("oauth: Cannot retrieve authentication info from server - %s", err))
		return
	}
	var ai AuthInfo
	if err = yaml.Unmarshal(pyld.AsBytes(), &ai); err != nil {
		cobra.CheckErr(fmt.Sprintf("oauth: Cannot parse authentication info from server. - %s", err))
		return
	}
	if ai.Version != 1 {
		cobra.CheckErr("oauth: Client out of date: Please update this application")
		return
	}
	providers := ai.ProviderList.AuthProviders
	defProvider := ai.ProviderList.DefaultProviderId
	if provider, ok := providers[defProvider]; ok {
		return verifyProviderInfo(&provider)
	}
	if defProvider != "" {
		cobra.CheckErr(fmt.Sprintf("oauth: Undeclared authentication provider '%s' returned", defProvider))
		return
	}
	// If no default provider is given, just pick the first one
	for _, p := range providers {
		return verifyProviderInfo(&p)
	}
	cobra.CheckErr("oauth: Cannot extract a suitable authentication provider")
	return // never get here
}

func verifyProviderInfo(p *AuthProvider) *AuthProvider {
	f := func(name string, urls string) {
		if _, e := url.ParseRequestURI(urls); e != nil {
			cobra.CheckErr(fmt.Sprintf("oauth: Authentication provider's %s '%s' is not a valid URL - %s", name, urls, e))
		}
	}
	f("LoginURL", p.LoginURL)
	f("TokenURL", p.TokenURL)
	f("CodeURL", p.CodeURL)
	f("JwksURL", p.JwksURL)
	return p
}

func requestDeviceCode(authProvider *AuthProvider) (code *DeviceCode) {
	adpt := CreateAdapter(false)
	params := url.Values{
		"client_id": {authProvider.ClientID},
		"scope":     {authProvider.scopes},
		"audience":  {authProvider.audience},
	}
	pyld, err := (*adpt).PostForm(NewTimeoutContext(), authProvider.CodeURL, params, nil, logger)
	if err != nil {
		cobra.CheckErr("oauth: Error while requesting device code from authentication provider")
		return
	}

	var dc DeviceCode
	if err = pyld.AsType(&dc); err != nil {
		logger.Error("while parsing 'DeviceCode'", log.String("pyld", string(pyld.AsBytes())))
		cobra.CheckErr("oauth: Cannot understand device information returned from authentication provider")
		return
	}
	return &dc
}

func waitForTokens(authProvider *AuthProvider, deviceCode *DeviceCode, ctxt *Context) *deviceTokenResponse {
	// We keep requesting until we're told not to by the server (too much time elapsed
	// for the user to login
	startTime := time.Now()
	lastElapsedTime := int64(0)

	params := url.Values{
		"device_code": {deviceCode.DeviceCode},
	}
	for {
		tokenResponse := getTokenResponse(authProvider, params, ctxt, true)
		logger.Debug("oauth: token response", log.Reflect("tr", tokenResponse))
		if tokenResponse.ErrorString == "" {
			return &tokenResponse
		}

		switch tokenResponse.ErrorString {
		case "authorization_pending":
			// No op - we're waiting on the user to open the link and login
		case "slow_down":
			// We're polling too fast, we should be using the interval supplied in the initial
			// device code request response, but the server has complained, we're going to increase
			// the wait interval
			deviceCode.Interval *= 2
		default:
			cobra.CheckErr(fmt.Sprintf("oauth: Authentication provider returned unexpected error '%s'", tokenResponse.ErrorString))
		}

		elapsedTime := int64(time.Since(startTime).Seconds())
		if elapsedTime/60 != lastElapsedTime/60 {
			fmt.Printf("... Time remaining: %d seconds\n", deviceCode.ExpiresIn-elapsedTime)
		}
		lastElapsedTime = elapsedTime

		// We sleep until we're allowed to poll again
		time.Sleep(time.Duration(deviceCode.Interval) * time.Second)
	}
}

func ParseIDToken(tokenResponse *deviceTokenResponse, ctxt *Context, jwksURL string) {
	// Lookup the public key to verify the signature (and check we have a valid token)

	// TODO: Download and cache the jwks data rather than download it on every login / token
	// refresh
	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{})
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot load the JWKS - %s", err))
	}
	idToken, err := jwt.ParseWithClaims(tokenResponse.IDToken, &CustomIdClaims{}, jwks.Keyfunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenUsedBeforeIssued) {
			// let's wait a bit and try again as this is most likely due to clock shifts as we immediately check
			// token after it has been created.
			logger.Info("oauth: Waiting a few seconds as token is not valid yet")
			time.Sleep(time.Duration(3 * time.Second))
			ParseIDToken(tokenResponse, ctxt, jwksURL)
			return
		} else if errors.Is(err, jwt.ErrTokenMalformed) {
			cobra.CheckErr(fmt.Sprintf("malformed ID Token received - %s", err))
		} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			// Token is either expired or not active yet
			cobra.CheckErr(fmt.Sprintf("expired ID Token received - %s", err))
		} else {
			cobra.CheckErr(fmt.Sprintf("cannot verify ID token - %s", err))
		}
	}

	if idToken == nil {
		cobra.CheckErr("Should never happen. No 'idToken' and no error")
	}
	if claims, ok := idToken.Claims.(*CustomIdClaims); ok && idToken.Valid {
		// Save the data from the ID token into the config/context
		ctxt.AccountName = claims.Name
		ctxt.Email = claims.Email
		ctxt.AccountNickName = claims.Nickname
		ctxt.AccountID = fmt.Sprintf("urn:%s:account:%s", URN_PREFIX, claims.AccountID)
		providerID := claims.ProviderID
		if providerID == "" {
			providerID = claims.AccountID
		}
		ctxt.ProviderID = fmt.Sprintf("urn:%s:provider:%s", URN_PREFIX, providerID)
	}
}

func login(_ *cobra.Command, args []string) {
	ctxt := GetActiveContext() // will always return ctxt or have already failed
	authProvider := getLoginInformation(ctxt)

	// offline_access is required for the refresh tokens to be sent through
	authProvider.scopes = "openid profile email offline_access"
	authProvider.grantType = "urn:ietf:params:oauth:grant-type:device_code"
	// TODO: Shouldn't that come from the server?
	authProvider.audience = "https://api.ivcap.net/"

	// First request a device code for this command line tool
	deviceCode := requestDeviceCode(authProvider)

	// Show QR code for authenticating via a web browser
	qrCode, err := qrcode.New(deviceCode.VerificationURLComplete, qrcode.Medium)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("cannot create QR code - %s", err))
	}
	qrCodeStrings := qrCode.ToSmallString(true)

	fmt.Println(string(qrCodeStrings))
	fmt.Println("    LOGIN CODE: ", deviceCode.UserCode)
	fmt.Println()

	fmt.Println()
	fmt.Println("To login to the IVCAP Service, please go to: ", deviceCode.VerificationURLComplete)
	fmt.Println("or scan the QR Code to be taken to the login page")
	fmt.Println("Waiting for authorisation...")

	tokenResponse := waitForTokens(authProvider, deviceCode, ctxt)
	ParseIDToken(tokenResponse, ctxt, authProvider.JwksURL)

	ctxt.AccessToken = tokenResponse.AccessToken
	// Add a 10 second buffer to expiry to account for differences in clock time between client
	// server and message transport time (oauth2 library does the same thing)
	ctxt.AccessTokenExpiry = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn-10))
	ctxt.RefreshToken = tokenResponse.RefreshToken
	SetContext(ctxt, true)

	fmt.Printf("Success: You are authorised.\n")
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
