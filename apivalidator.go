// Package apivalidator validates HTTP requests and responses against API
// schemas in a variety of formats: OpenAPI 3.x / Swagger 2.0, RAML, API
// Blueprint, gRPC (.proto), CapnProto, Thrift, JSON Schema, and GraphQL SDL.
//
// Basic usage:
//
//	v := apivalidator.NewValidator()
//	schema, err := apivalidator.ParseSchemaFile(apivalidator.FormatOpenAPI3, "api.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	v.AddSchema(schema)
//
//	result, err := v.ValidateRequest(req)
//	if err != nil {
//	    // err == ErrNoMatchingSchema: no registered schema covers this endpoint
//	    log.Printf("no schema: %v", err)
//	    return
//	}
//	if !result.Valid {
//	    log.Printf("invalid: %s", result.Error())
//	}
package apivalidator

import (
	"net/http"
	"sync"
)

// Validator holds a set of registered schemas and validates requests/responses
// against them. It is safe for concurrent use after construction.
type Validator struct {
	cfg     *Config
	mu      sync.RWMutex
	schemas []Schema
}

// NewValidator constructs a new Validator with the given options.
func NewValidator(opts ...Option) *Validator {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	return &Validator{cfg: cfg}
}

// AddSchema registers a pre-parsed Schema. Returns v so calls can be chained.
func (v *Validator) AddSchema(s Schema) *Validator {
	v.mu.Lock()
	v.schemas = append(v.schemas, s)
	v.mu.Unlock()
	return v
}

// AddSchemaBytes parses a schema from raw bytes and registers it.
func (v *Validator) AddSchemaBytes(format Format, data []byte) (*Validator, error) {
	s, err := ParseSchema(format, data)
	if err != nil {
		return v, err
	}
	return v.AddSchema(s), nil
}

// AddSchemaString parses a schema from a string and registers it.
func (v *Validator) AddSchemaString(format Format, src string) (*Validator, error) {
	s, err := ParseSchemaString(format, src)
	if err != nil {
		return v, err
	}
	return v.AddSchema(s), nil
}

// AddSchemaFile parses a schema from a file on disk and registers it.
func (v *Validator) AddSchemaFile(format Format, path string) (*Validator, error) {
	s, err := ParseSchemaFile(format, path)
	if err != nil {
		return v, err
	}
	return v.AddSchema(s), nil
}

// ValidateRequest validates req against all registered schemas.
//
// The matching schema is selected automatically based on Content-Type, path
// structure, and registered schema formats (see "Schema routing" in the docs).
//
// Returns ErrNoMatchingSchema when no registered schema covers this endpoint.
// Any other non-nil error indicates a failure to attempt validation (body read
// error, schema internal error, etc.).
//
// Body buffering: if body validation is enabled, req.Body is read and replaced
// with a re-readable buffer so the caller can still read the original bytes.
func (v *Validator) ValidateRequest(req *http.Request) (*ValidationResult, error) {
	v.mu.RLock()
	schemas := v.schemas
	v.mu.RUnlock()

	return v.dispatch(req, nil, schemas)
}

// ValidateResponse validates resp against registered schemas. req is the
// original request that produced resp; it is used to determine which endpoint
// and method apply (and therefore which response schema and status codes are
// valid).
//
// Body buffering: resp.Body is read and replaced with a re-readable buffer.
func (v *Validator) ValidateResponse(resp *http.Response, req *http.Request) (*ValidationResult, error) {
	v.mu.RLock()
	schemas := v.schemas
	v.mu.RUnlock()

	return v.dispatch(req, resp, schemas)
}

// ValidateResponseBody validates a raw response body without the original
// request. Useful when only response captures are available (e.g. log replay).
// contentType is used for schema routing and body decoding.
func (v *Validator) ValidateResponseBody(body []byte, contentType string) (*ValidationResult, error) {
	v.mu.RLock()
	schemas := v.schemas
	v.mu.RUnlock()

	return v.dispatchBody(body, contentType, schemas)
}

// ValidateRequest is a one-shot convenience function for callers with a single
// schema and no need for a persistent validator.
func ValidateRequest(req *http.Request, s Schema) (*ValidationResult, error) {
	return NewValidator().AddSchema(s).ValidateRequest(req)
}

// ValidateResponse is a one-shot convenience function for callers with a
// single schema and no need for a persistent validator.
func ValidateResponse(resp *http.Response, req *http.Request, s Schema) (*ValidationResult, error) {
	return NewValidator().AddSchema(s).ValidateResponse(resp, req)
}
