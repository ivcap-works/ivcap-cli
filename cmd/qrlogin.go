package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var qrLoginCmd = &cobra.Command{
	Use:   "qrlogin codeURL tokenURL clientID",
	Short: "Authenticate with a specific deployment/context",
	Run:   loginQR,
}

type QRAuthInfo struct {
	codeURL      string
	tokenURL     string
	clientID     string
	refreshToken string
	audience     string
	scopes       string
	grantType    string
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
	Name          string `json:"name,omitempty"`
	Nickname      string `json:"nickname,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Picture       string `json:"picture,omitempty"`
	jwt.RegisteredClaims
}

type deviceTokenResponse struct {
	*oauth2.Token
	IDToken     string `json:"id_token,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
	ErrorString string `json:"error,omitempty"`
}

// If we already have a refresh token, we don't need to go through the whole device code
// interaction. We can simply use the refresh token to request another access token.
func refreshAccessToken() error {
	ctxt := GetActiveContext()

	accessTokenExpiry := ctxt.AccessTokenExpiry
	if time.Now().After(accessTokenExpiry) {
		// Access token has expired, we have to refresh it
		authInfo := QRAuthInfo{}
		authInfo.tokenURL = ctxt.TokenURL
		authInfo.grantType = "refresh_token"
		authInfo.clientID = ctxt.ClientID
		authInfo.refreshToken = ctxt.RefreshToken

		if (authInfo.tokenURL != "") && (authInfo.clientID != "") && (authInfo.refreshToken != "") {

			response, err := http.PostForm(authInfo.tokenURL,
				url.Values{"grant_type": {authInfo.grantType},
					"client_id":     {authInfo.clientID},
					"refresh_token": {authInfo.refreshToken}})

			if err != nil {
				return fmt.Errorf("Cannot refresh access token - %s", err)
			}

			var tokenResponse deviceTokenResponse
			jsonDecoder := json.NewDecoder(response.Body)
			if err := jsonDecoder.Decode(&tokenResponse); err != nil {
				return fmt.Errorf("Cannot decode token response - %s", err)
			}

			switch tokenResponse.ErrorString {
			case "authorization_pending":
				// No op - we're waiting on the user to open the link and login
			case "expired_token":
				return fmt.Errorf("The login process was not completed in time - please login again")
			case "access_denied":
				return fmt.Errorf("Could not login - access was denied")
			case "":
				// No Errors:
				ctxt.AccessToken = tokenResponse.AccessToken
				// Add a 10 second buffer to expiry to account for differences in clock time between client
				// server and message transport time (oauth2 library does the same thing)
				ctxt.AccessTokenExpiry = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn-10))

				// We also get an updated ID token, let's make sure we have the latest info
				ParseIDToken(&tokenResponse, ctxt)

				fmt.Println(fmt.Sprintf("Successfully acquired new access token. Expiry: %s", ctxt.AccessTokenExpiry))

				SetContext(ctxt, true)
			}

		} // Access token has not expired, let's just use it
	}

	return nil

}

func requestDeviceCode(client *http.Client, authInfo *QRAuthInfo) (*DeviceCode, error) {
	response, err := http.PostForm(authInfo.codeURL,
		url.Values{"client_id": {authInfo.clientID},
			"scope":    {authInfo.scopes},
			"audience": {authInfo.audience}})

	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot request authentication device code - %s", err))
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP Request Error: Device code request returned %v (%v)",
			response.StatusCode, http.StatusText(response.StatusCode))
	}

	// Read the device code from the body of the returned response
	var deviceCode DeviceCode
	jsonDecoder := json.NewDecoder(response.Body)
	if err := jsonDecoder.Decode(&deviceCode); err != nil {
		return nil, err
	}

	return &deviceCode, nil
}

func waitForTokens(client *http.Client, authInfo *QRAuthInfo, deviceCode *DeviceCode) (*deviceTokenResponse, error) {
	// We keep requesting until we're told not to by the server (too much time elapsed
	// for the user to login
	startTime := time.Now()
	lastElapsedTime := int64(0)
	for {
		response, err := http.PostForm(authInfo.tokenURL,
			url.Values{"grant_type": {authInfo.grantType},
				"client_id":   {authInfo.clientID},
				"device_code": {deviceCode.DeviceCode}})

		if err != nil {
			return nil, fmt.Errorf("Cannot request tokens - %s", err)
		}

		// Auth0 unfortunately returns statusforbidden while we're waiting for a token, so
		// we can't just exist here if != statusOk
		if (response.StatusCode != http.StatusOK) && (response.StatusCode != http.StatusForbidden) {
			return nil, fmt.Errorf("HTTP Request Error: Token Request returned %v (%v)",
				response.StatusCode,
				http.StatusText(response.StatusCode))
		}

		/*
			responseRaw, err := io.ReadAll(response.Body)
			fmt.Printf("%s", string(responseRaw))

			var dat map[string]interface{}
			if err := json.Unmarshal(responseRaw, &dat); err != nil {
				panic(err)
			}
			fmt.Println(dat)
			if dat["error"] != nil {
				errorvalue := dat["error"].(string)
				if errorvalue != "" {
					fmt.Println(errorvalue)
					time.Sleep(time.Duration(deviceCode.Interval) * time.Second)
					continue
				}
			}
		*/

		var tokenResponse deviceTokenResponse
		jsonDecoder := json.NewDecoder(response.Body)
		if err := jsonDecoder.Decode(&tokenResponse); err != nil {
			return nil, fmt.Errorf("Cannot decode token response - %s", err)
		}

		switch tokenResponse.ErrorString {
		case "authorization_pending":
			// No op - we're waiting on the user to open the link and login
		case "slow_down":
			// We're polling too fast, we should be using the interval supplied in the initial
			// device code request response, but the server has complained, we're going to increase
			// the wait interval
			deviceCode.Interval *= 2
		case "expired_token":
			return nil, fmt.Errorf("The login process was not completed in time - please login again")
		case "access_denied":
			return nil, fmt.Errorf("Could not login - access was denied")
		case "":
			// No Errors:
			return &tokenResponse, nil
		}

		elapsedTime := int64(time.Since(startTime).Seconds())
		if elapsedTime/60 != lastElapsedTime/60 {
			fmt.Println(fmt.Sprintf("Time remaining: %d seconds", deviceCode.ExpiresIn-elapsedTime))
		}
		lastElapsedTime = elapsedTime

		// We sleep until we're allowed to poll again
		time.Sleep(time.Duration(deviceCode.Interval) * time.Second)
	}

}

func ParseIDToken(tokenResponse *deviceTokenResponse, ctxt *Context) error {
	// Lookup the public key to verify the signature (and check we have a valid token)
	jwksURL := "https://ivap.au.auth0.com/.well-known/jwks.json"

	// Todo look at keyfunc options, to get a cancellable context
	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{})

	idToken, err := jwt.ParseWithClaims(tokenResponse.IDToken, &CustomIdClaims{}, jwks.Keyfunc)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return fmt.Errorf("Malformed ID Token Received - %s", err)
		} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			// Token is either expired or not active yet
			return fmt.Errorf("Expired ID Token Received - %s", err)
		} else {
			return fmt.Errorf("Cannot verify ID token - %s", err)
		}
	}

	if idToken != nil {
		if claims, ok := idToken.Claims.(*CustomIdClaims); ok && idToken.Valid {
			// Save the data from the ID token into the config/context
			ctxt.AccountName = claims.Name
			ctxt.Email = claims.Email
			ctxt.AccountNickName = claims.Nickname
		}
	}

	return nil
}

func loginQR(_ *cobra.Command, args []string) {
	ctxt := GetActiveContext()

	if ctxt == nil {
		cobra.CheckErr("Invalid config set. Please set a valid config with the config command.")
		return
	}
	authInfo := QRAuthInfo{}

	// offline_access is required for the refresh tokens to be sent through
	authInfo.scopes = "openid profile email offline_access"
	authInfo.grantType = "urn:ietf:params:oauth:grant-type:device_code"
	authInfo.audience = "https://ivap.au.auth0.com/api/v2/"

	if len(args) > 1 {
		authInfo.codeURL = args[0]
		authInfo.tokenURL = args[1]
		authInfo.clientID = args[2]
	} else {
		if ctxt != nil {
			if ctxt.codeURL != "" {
				authInfo.codeURL = ctxt.codeURL
			} else {
				cobra.CheckErr("Missing 'codeURL'")
				return
			}
			if ctxt.TokenURL != "" {
				authInfo.tokenURL = ctxt.TokenURL
			} else {
				cobra.CheckErr("Missing 'tokenURL'")
				return
			}
			if ctxt.ClientID != "" {
				authInfo.clientID = ctxt.ClientID
				return
			} else {
				cobra.CheckErr("Missing 'clientID'")
				return
			}
		}
	}

	httpClient := http.DefaultClient

	// First request a device code for this command line tool
	deviceCode, err := requestDeviceCode(httpClient, &authInfo)

	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot request authentication device code - %s", err))
		return
	}

	qrCode, err := qrcode.New(deviceCode.VerificationURLComplete, qrcode.Medium)
	qrCodeStrings := qrCode.ToSmallString(true)

	fmt.Println(string(qrCodeStrings))
	fmt.Println("    LOGIN CODE: ", deviceCode.UserCode)
	fmt.Println()

	fmt.Println()
	fmt.Println("To login to the IVCAP Service, please go to: ", deviceCode.VerificationURLComplete)
	fmt.Println("or scan the QR Code to be taken to the login page")
	fmt.Println("Waiting for authorisation...")

	tokenResponse, err := waitForTokens(httpClient, &authInfo, deviceCode)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot request authorisation tokens - %s", err))
		return
	}

	fmt.Println(fmt.Sprintf("Command Line Tool Authorised."))
	err = ParseIDToken(tokenResponse, ctxt)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot parse identity information - %s", err))
		return
	}

	ctxt.ClientID = authInfo.clientID
	ctxt.TokenURL = authInfo.tokenURL
	ctxt.AccessToken = tokenResponse.AccessToken
	// Add a 10 second buffer to expiry to account for differences in clock time between client
	// server and message transport time (oauth2 library does the same thing)
	ctxt.AccessTokenExpiry = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn-10))
	ctxt.RefreshToken = tokenResponse.RefreshToken
	SetContext(ctxt, true)

	// fmt.Println(fmt.Sprintf("Access Token Expires at: %s", ctxt.AccessTokenExpiry))
}

func init() {
	rootCmd.AddCommand(qrLoginCmd)
}
