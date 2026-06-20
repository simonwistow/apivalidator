package apivalidator

import "net/http"

// schemaValidator is the interface each format-specific schema must implement.
// Implementations live in the schemas/* sub-packages and are registered via
// their init() functions (same plugin pattern as apianalyzer algorithms).
type schemaValidator interface {
	ValidateRequest(req *http.Request, cfg *Config) (*ValidationResult, error)
	ValidateResponse(resp *http.Response, req *http.Request, cfg *Config) (*ValidationResult, error)
}

// schemaImpl returns the schemaValidator implementation for s, if s implements it.
// Schemas returned by ParseSchema always implement schemaValidator internally.
func schemaImpl(s Schema) (schemaValidator, bool) {
	impl, ok := s.(schemaValidator)
	return impl, ok
}
