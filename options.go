package apivalidator

import "log/slog"

// MultiSchemaPolicy controls what happens when multiple registered schemas
// match the same request.
type MultiSchemaPolicy int

const (
	// PolicyAll requires every matching schema to pass. This is the default
	// and is the strictest mode — useful for contract testing.
	PolicyAll MultiSchemaPolicy = iota

	// PolicyAny requires at least one matching schema to pass. Useful when
	// multiple versions of an API schema are registered simultaneously during
	// a migration (e.g. v1 and v2 both registered; requests valid under either
	// version are accepted).
	PolicyAny
)

// JSONSchemaScope controls which traffic a standalone FormatJSONSchema applies to
// when registered without an enclosing OpenAPI/RAML schema.
type JSONSchemaScope int

const (
	// JSONSchemaBoth applies the schema to both request and response bodies.
	// This is the default.
	JSONSchemaBoth JSONSchemaScope = iota

	// JSONSchemaRequest applies the schema to request bodies only.
	JSONSchemaRequest

	// JSONSchemaResponse applies the schema to response bodies only.
	JSONSchemaResponse
)

// Config holds the resolved configuration for a Validator. It is passed to
// schema-specific validators so they can respect the caller's settings without
// each needing to re-parse the Option list.
type Config struct {
	MaxBodyBytes        int64
	StrictMode          bool
	IgnoreUnknown       bool
	MultiSchemaPolicy   MultiSchemaPolicy
	Logger              *slog.Logger
	ValidateBody        bool
	ValidateQueryParams bool
	ValidateHeaders     bool
	JSONSchemaScope     JSONSchemaScope
}

func defaultConfig() *Config {
	return &Config{
		MaxBodyBytes:        4 << 20, // 4 MiB
		StrictMode:          false,
		IgnoreUnknown:       false,
		MultiSchemaPolicy:   PolicyAll,
		ValidateBody:        true,
		ValidateQueryParams: true,
		ValidateHeaders:     false,
		JSONSchemaScope:     JSONSchemaBoth,
	}
}

// Option is a functional option for configuring a Validator.
type Option func(*Config)

// WithMaxBodyBytes sets the maximum number of bytes the validator will buffer
// when reading a request or response body. Requests with larger bodies are
// rejected with ErrBodyTooLarge. Default is 4 MiB.
func WithMaxBodyBytes(n int64) Option {
	return func(c *Config) { c.MaxBodyBytes = n }
}

// WithStrictMode promotes unknown-endpoint misses from ErrNoMatchingSchema to
// a SeverityError Violation with code CodeUnknownEndpoint. Default is false.
func WithStrictMode(b bool) Option {
	return func(c *Config) { c.StrictMode = b }
}

// WithIgnoreUnknownEndpoints suppresses ErrNoMatchingSchema entirely: requests
// to endpoints not covered by any registered schema pass validation silently.
func WithIgnoreUnknownEndpoints() Option {
	return func(c *Config) { c.IgnoreUnknown = true }
}

// WithMultiSchemaPolicy sets the policy used when multiple schemas match the
// same request. Default is PolicyAll.
func WithMultiSchemaPolicy(p MultiSchemaPolicy) Option {
	return func(c *Config) { c.MultiSchemaPolicy = p }
}

// WithLogger sets the structured logger used for diagnostic output.
// If not set, all diagnostic logging is suppressed.
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithValidateBody enables or disables body validation. Default is true.
// Set to false for path-and-header-only validation (lower overhead).
func WithValidateBody(b bool) Option {
	return func(c *Config) { c.ValidateBody = b }
}

// WithValidateQueryParams enables or disables query parameter validation.
// Default is true.
func WithValidateQueryParams(b bool) Option {
	return func(c *Config) { c.ValidateQueryParams = b }
}

// WithValidateHeaders enables or disables header validation. Default is false
// because most API schemas do not declare headers explicitly.
func WithValidateHeaders(b bool) Option {
	return func(c *Config) { c.ValidateHeaders = b }
}

// WithJSONSchemaScope sets the scope for standalone FormatJSONSchema schemas.
// Default is JSONSchemaBoth (applied to both request and response bodies).
func WithJSONSchemaScope(s JSONSchemaScope) Option {
	return func(c *Config) { c.JSONSchemaScope = s }
}
