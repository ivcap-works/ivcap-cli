// Adapted from https://github.com/maxott/magda-cli/blob/main/pkg/adapter/adapter.go
package adapter

import (
	"context"
	"io"
	"net/http"

	log "go.uber.org/zap"
)

type ResponseHandler func(response *http.Response, path string, logger *log.Logger) (err error)

type Adapter interface {
	Head(ctxt context.Context, path string, headers *map[string]string, logger *log.Logger) (Payload, error)
	Get(ctxt context.Context, path string, logger *log.Logger) (Payload, error)
	Get2(ctxt context.Context, path string, headers *map[string]string, respHandler ResponseHandler, logger *log.Logger) error
	Post(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error)
	Put(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error)
	Patch(ctxt context.Context, path string, body io.Reader, length int64, headers *map[string]string, logger *log.Logger) (Payload, error)
	Delete(ctxt context.Context, path string, logger *log.Logger) (Payload, error)
	ClearAuthorization() // no longer add authorization info to calls
	SetUrl(url string)
	GetPath(url string) (path string, err error)
}

type Payload interface {
	// IsObject() bool
	AsType(r interface{}) error
	AsObject() (map[string]interface{}, error)
	AsArray() ([]interface{}, error)
	AsBytes() []byte
	Header(key string) string
	StatusCode() int
}
