// Package accountsapi holds Go data models for the ivcap-accounts HTTP API,
// generated from the OpenAPI spec ivcap-id publishes at
// https://id.<domain>/openapi.json (the Goa designs are the source of truth).
// Do not edit models.gen.go by hand.
//
// The spec is vendored here as openapi.json (refresh with `make sync-specs`);
// regenerate the models with `make gen`. Keep the oapi-codegen version in step
// with OAPI_CODEGEN_VERSION in the Makefile.
//
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config cfg.yaml openapi.json
package accountsapi
