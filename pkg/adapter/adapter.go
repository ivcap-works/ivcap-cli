// Program to create, update & delete aspect schemas in cayp
// Adapted from https://github.com/maxott/cayp-cli/blob/main/pkg/adapter/adapter.go
package adapter

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	log "go.uber.org/zap"
)

type ConnectionCtxt struct {
	URL        string
	JwtToken   string
	TimeoutSec int
}

func CreateJwtToken(userID *string, signingSecret *string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": *userID,
		"iat":    time.Now().Unix(),
	})
	return token.SignedString([]byte(*signingSecret))
}

func RestAdapter(connCtxt ConnectionCtxt) Adapter {
	return &restAdapter{connCtxt}
}

type IAdapterError interface {
	Error() string
	Path() string
}
type AdapterError struct {
	path string
}

func (e *AdapterError) Path() string { return e.path }

func (e *AdapterError) Error() string { return "Generic cayp adapter error" }

type MissingUrlError struct {
	AdapterError
}

func (e MissingUrlError) Error() string { return "Missing deployment URL" }

type ResourceNotFoundError struct {
	AdapterError
}

func (e ResourceNotFoundError) Error() string { return "Resource not found" }

type UnauthorizedError struct {
	AdapterError
}

func (e *UnauthorizedError) Error() string { return "Unauthorized access" }

type ApiError struct {
	AdapterError
	StatusCode int
	Message    string
}

func (e *ApiError) Error() string { return e.Message }

type ClientError struct {
	AdapterError
	err error
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("while connecting to cayp registry - %s", e.err.Error())
}

type restAdapter struct {
	ctxt ConnectionCtxt
}

func (a *restAdapter) Head(ctxt context.Context, path string, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "HEAD", path, nil, -1, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Get(ctxt context.Context, path string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "GET", path, nil, -1, nil, &a.ctxt, nil, logger)
}

func (a *restAdapter) Get2(ctxt context.Context, path string, headers *map[string]string, respHandler ResponseHandler, logger *log.Logger) error {
	_, err := Connect(ctxt, "GET", path, nil, -1, headers, &a.ctxt, respHandler, logger)
	return err
}

func (a *restAdapter) Post(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "POST", path, body, length, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Put(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "PUT", path, body, length, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Patch(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "PATCH", path, body, length, headers, &a.ctxt, nil, logger)
}

func (a *restAdapter) Delete(ctxt context.Context, path string, logger *log.Logger) (Payload, error) {
	return Connect(ctxt, "DELETE", path, nil, -1, nil, &a.ctxt, nil, logger)
}

func (a *restAdapter) ClearAuthorization() {
	a.ctxt.JwtToken = ""
}

func (a *restAdapter) SetUrl(url string) {
	a.ctxt.URL = url
}

func (a *restAdapter) GetPath(url string) (path string, err error) {
	if strings.HasPrefix(url, a.ctxt.URL) {
		path = url[len(a.ctxt.URL):]
	} else {
		err = fmt.Errorf("url '%s' is not for this deployment '%s'", url, a.ctxt.URL)
	}
	return
}

func Connect(
	ctxt context.Context,
	method string,
	path string,
	body io.Reader,
	length int64,
	headers *map[string]string,
	connCtxt *ConnectionCtxt,
	respHandler ResponseHandler,
	logger *log.Logger,
) (Payload, error) {
	logger = logger.With(log.String("method", method), log.String("path", path))
	if connCtxt.URL == "" {
		//logger.Error("Missing 'host'")
		return nil, &MissingUrlError{AdapterError{path}}
	}
	url := connCtxt.URL + path
	logger = logger.With(log.String("url", url))
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		logger.Error("Creating http request", log.Error(err))
		return nil, &ClientError{AdapterError{path}, err}
	}
	if length > 0 {
		req.ContentLength = length
	}
	contentType := "application/json"
	if headers != nil {
		if ct, ok := (*headers)["Content-Type"]; ok {
			contentType = ct
		}
	}
	if length > 0 {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Cache-Control", "no-cache")
	if connCtxt.JwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+connCtxt.JwtToken)
	}
	if headers != nil {
		for key, val := range *headers {
			if key != "Content-Type" {
				logger.Debug("header", log.String("key", key), log.String("val", val))
				// v := base64.StdEncoding.EncodeToString([]byte(val))
				req.Header.Set(key, val)
			}
		}
	}

	client := &http.Client{Timeout: time.Second * time.Duration(connCtxt.TimeoutSec)}
	logger.Debug("calling api", log.Reflect("headers", req.Header))
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("HTTP request failed.", log.Error(err), log.Reflect("err2", err))
		return nil, &ClientError{AdapterError{path}, err}
	}
	defer resp.Body.Close()

	if respHandler != nil {
		err := respHandler(resp)
		return nil, err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("Accessing response body failed.", log.Error(err))
		return nil, &ClientError{AdapterError{path}, err}
	}
	logger.Debug("successful reply", log.Int("statusCode", resp.StatusCode),
		log.Int("body-length", len(respBody)), log.Reflect("headers", resp.Header))

	if resp.StatusCode >= 300 {
		if len(respBody) > 0 {
			logger = logger.With(log.ByteString("body", respBody))
		}
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, &ResourceNotFoundError{AdapterError{path}}
		case http.StatusUnauthorized:
			return nil, &UnauthorizedError{AdapterError{path}}
		default:
			logger.Warn("HTTP response", log.Int("statusCode", resp.StatusCode))
			return nil, &ApiError{
				AdapterError{path},
				resp.StatusCode,
				string(respBody),
			}
		}

		//ResourceNotFoundError
	}
	return ToPayload(respBody, resp, logger)
}
