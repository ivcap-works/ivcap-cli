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
	Use:   "qrlogin",
	Short: "Authenticate with a specific deployment/context",
	Run:   loginQR,
}

type QRAuthInfo struct {
	LoginURL  string `json:"login-url"`
	TokenURL  string `json:"token-url"`
	CodeURL   string `json:"code-url"`
	JwksURL   string `json:"jwks-url"`
	ClientID  string `json:"client-id"`
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
	Name          string `json:"name,omitempty"`
	Nickname      string `json:"nickname,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Picture       string `json:"picture,omitempty"`
	AccountID     string `json:"ivap/claims/account-id,omitempty"`
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
func refreshAccessToken() (accessToken string, err error) {
	ctxt := GetActiveContext()

	accessTokenExpiry := ctxt.AccessTokenExpiry
	if time.Now().After(accessTokenExpiry) {
		if ctxt.RefreshToken == "" {
			// We don't have a refresh token for this context, so we fail early
			return "", fmt.Errorf("Could not login - invalid credentials. Please use the login command to refresh your credentials")
		}

		authInfo, err := getLoginInformation(http.DefaultClient, ctxt)

		if err != nil {
			cobra.CheckErr(fmt.Sprintf("Could not connect to %s - %s", ctxt.URL, err))
			return "", err
		}

		// Access token has expired, we have to refresh it
		authInfo.grantType = "refresh_token"

		if (authInfo.TokenURL != "") && (authInfo.ClientID != "") {

			response, err := http.PostForm(authInfo.TokenURL,
				url.Values{"grant_type": {authInfo.grantType},
					"client_id":     {authInfo.ClientID},
					"refresh_token": {ctxt.RefreshToken}})

			if err != nil {
				return "", fmt.Errorf("Cannot refresh access token - %s", err)
			}

			var tokenResponse deviceTokenResponse
			jsonDecoder := json.NewDecoder(response.Body)
			if err := jsonDecoder.Decode(&tokenResponse); err != nil {
				return "", fmt.Errorf("Cannot decode token response - %s", err)
			}

			switch tokenResponse.ErrorString {
			case "authorization_pending":
				// No op - we're waiting on the user to open the link and login
			case "expired_token":
				return "", fmt.Errorf("The login process was not completed in time - please login again")
			case "access_denied":
				return "", fmt.Errorf("Could not login - access was denied")
			case "invalid_grant":
				return "", fmt.Errorf("Could not login - expired credentials. Please use the login command to refresh your credentials")
			case "":
				// No Errors:
				ctxt.AccessToken = tokenResponse.AccessToken
				// Add a 10 second buffer to expiry to account for differences in clock time between client
				// server and message transport time (oauth2 library does the same thing)
				ctxt.AccessTokenExpiry = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn-10))

				// We also get an updated ID token, let's make sure we have the latest info
				ParseIDToken(&tokenResponse, ctxt, authInfo.JwksURL)

				fmt.Println(fmt.Sprintf("Successfully acquired new access token. Expiry: %s", ctxt.AccessTokenExpiry))

				SetContext(ctxt, true)
			}

		} // Access token has not expired, let's just use it
	}

	return ctxt.AccessToken, nil
}

func getLoginInformation(client *http.Client, ctxt *Context) (authInfo *QRAuthInfo, err error) {
	if ctxt == nil {
		return nil, fmt.Errorf("Invalid config set. Please set a valid config with the config command.")
	}

	response, err := http.Get(ctxt.URL + "/logininfo")

	if err != nil {
		return nil, fmt.Errorf("Cannot request login info - %s", err)
	}

	jsonDecoder := json.NewDecoder(response.Body)
	if err := jsonDecoder.Decode(&authInfo); err != nil {
		return nil, err
	}

	return authInfo, nil
}

func requestDeviceCode(client *http.Client, authInfo *QRAuthInfo) (*DeviceCode, error) {
	response, err := http.PostForm(authInfo.CodeURL,
		url.Values{"client_id": {authInfo.ClientID},
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
		response, err := http.PostForm(authInfo.TokenURL,
			url.Values{"grant_type": {authInfo.grantType},
				"client_id":   {authInfo.ClientID},
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

func ParseIDToken(tokenResponse *deviceTokenResponse, ctxt *Context, jwksURL string) error {
	// Lookup the public key to verify the signature (and check we have a valid token)

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
			ctxt.AccountID = claims.AccountID
		}
	}

	return nil
}

func loginQR(_ *cobra.Command, args []string) {
	ctxt := GetActiveContext()

	httpClient := http.DefaultClient

	if ctxt == nil {
		cobra.CheckErr("Invalid config set. Please set a valid config with the config command.")
		return
	}
	authInfo, err := getLoginInformation(httpClient, ctxt)

	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Could not connect to %s to login - %s", ctxt.URL, err))
		return
	}

	// offline_access is required for the refresh tokens to be sent through
	authInfo.scopes = "openid profile email offline_access"
	authInfo.grantType = "urn:ietf:params:oauth:grant-type:device_code"
	authInfo.audience = "https://api.ivcap.net/"

	// First request a device code for this command line tool
	deviceCode, err := requestDeviceCode(httpClient, authInfo)

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

	tokenResponse, err := waitForTokens(httpClient, authInfo, deviceCode)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot request authorisation tokens - %s", err))
		return
	}

	fmt.Println(fmt.Sprintf("Command Line Tool Authorised."))
	err = ParseIDToken(tokenResponse, ctxt, authInfo.JwksURL)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Cannot parse identity information - %s", err))
		return
	}

	ctxt.ClientID = authInfo.ClientID
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
