// Adapted from https://github.com/maxott/magda-cli/blob/main/pkg/adapter/adapter.go
package adapter

import (
	"context"
	"io"

	log "go.uber.org/zap"
)

type Adapter interface {
	Get(ctxt context.Context, path string, logger *log.Logger) (Payload, error)
	Post(ctxt context.Context, path string, body io.Reader, headers *map[string]string, logger *log.Logger) (Payload, error)
	Put(ctxt context.Context, path string, body io.Reader, headers *map[string]string, logger *log.Logger) (Payload, error)
	Patch(ctxt context.Context, path string, body io.Reader, logger *log.Logger) (Payload, error)
	Delete(ctxt context.Context, path string, logger *log.Logger) (Payload, error)
	ClearAuthorization() // no longer add authorization info to calls
}

type Payload interface {
	// IsObject() bool
	AsType(r interface{}) error
	AsObject() (map[string]interface{}, error)
	AsArray() ([]interface{}, error)
	AsBytes() []byte
}
