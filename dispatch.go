package apivalidator

import (
	"net/http"
	"strings"

	"github.com/simonwistow/apivalidator/internal/route"
)

// bodyValidator is an optional interface implemented by schemas that can
// validate a raw body without any request context. JSON Schema implements this;
// format-specific schemas (OpenAPI, gRPC, etc.) do not because they need the
// request path to select the right operation/message type.
type bodyValidator interface {
	ValidateBody(body []byte) *ValidationResult
}

// dispatch selects matching schemas and runs validation. When resp is nil,
// request validation is performed; otherwise response validation runs.
func (v *Validator) dispatch(req *http.Request, resp *http.Response, schemas []Schema) (*ValidationResult, error) {
	transport := route.DetectTransport(req)

	// Filter schemas by transport compatibility.
	candidates := v.candidatesFor(transport, schemas)

	if len(candidates) == 0 {
		return v.noSchemaResult(false)
	}

	// Run validation against each candidate.
	var results []*ValidationResult
	for _, s := range candidates {
		var (
			r   *ValidationResult
			err error
		)
		if resp == nil {
			r, err = v.validateRequestWith(req, s)
		} else {
			r, err = v.validateResponseWith(resp, req, s)
		}
		if err == ErrNoMatchingSchema {
			continue
		}
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	if len(results) == 0 {
		// Schemas matched the transport but none had a route for this path.
		return v.noSchemaResult(true)
	}

	return v.mergeResults(results), nil
}

// dispatchBody runs response-body-only validation (no request context).
// Only schemas that implement bodyValidator (currently FormatJSONSchema) are
// applicable here, since other formats require a request path to route to the
// correct operation or message type.
func (v *Validator) dispatchBody(body []byte, contentType string, schemas []Schema) (*ValidationResult, error) {
	// Normalise content-type: strip parameters and lower-case.
	ct := strings.ToLower(contentType)
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}

	// Only validate JSON bodies (empty content-type is treated permissively).
	if ct != "" && ct != "application/json" {
		return v.noSchemaResult(false)
	}

	// If JSONSchemaScope is request-only, skip response body validation.
	if v.cfg.JSONSchemaScope == JSONSchemaRequest {
		return v.noSchemaResult(false)
	}

	var results []*ValidationResult
	for _, s := range schemas {
		bv, ok := s.(bodyValidator)
		if !ok {
			continue
		}
		results = append(results, bv.ValidateBody(body))
	}

	if len(results) == 0 {
		return v.noSchemaResult(false)
	}

	return v.mergeResults(results), nil
}

// candidatesFor filters schemas to those compatible with the detected transport.
func (v *Validator) candidatesFor(transport route.Transport, schemas []Schema) []Schema {
	out := make([]Schema, 0, len(schemas))
	for _, s := range schemas {
		if schemaMatchesTransport(s.Format(), transport) {
			out = append(out, s)
		}
	}
	return out
}

// schemaMatchesTransport reports whether a schema format can handle a given transport.
func schemaMatchesTransport(f Format, t route.Transport) bool {
	switch t {
	case route.TransportGRPC, route.TransportGRPCWeb:
		return f == FormatGRPC
	case route.TransportGraphQL:
		return f == FormatGraphQL
	case route.TransportThrift:
		return f == FormatThrift
	case route.TransportREST, route.TransportUnknown:
		// Cap'n Proto has no universal Content-Type — match on REST/unknown transport.
		return f == FormatOpenAPI3 || f == FormatOpenAPI2 ||
			f == FormatRAML || f == FormatJSONSchema || f == FormatCapnProto
	default:
		return false
	}
}

// mergeResults combines multiple ValidationResults according to the configured
// MultiSchemaPolicy.
func (v *Validator) mergeResults(results []*ValidationResult) *ValidationResult {
	if len(results) == 1 {
		return results[0]
	}

	merged := &ValidationResult{MatchedSchema: "multiple"}

	switch v.cfg.MultiSchemaPolicy {
	case PolicyAny:
		// Valid if at least one result is valid.
		for _, r := range results {
			if r.Valid {
				merged.Valid = true
				merged.MatchedEndpoint = r.MatchedEndpoint
				return merged
			}
		}
		// None passed — collect all violations.
		merged.Valid = false
		for _, r := range results {
			merged.Violations = append(merged.Violations, r.Violations...)
		}

	default: // PolicyAll
		// Valid only if every result is valid.
		merged.Valid = true
		for _, r := range results {
			if !r.Valid {
				merged.Valid = false
				merged.Violations = append(merged.Violations, r.Violations...)
			}
		}
	}

	return merged
}

// validateRequestWith dispatches to the format-specific validator for a request.
func (v *Validator) validateRequestWith(req *http.Request, s Schema) (*ValidationResult, error) {
	impl, ok := schemaImpl(s)
	if !ok {
		return nil, ErrNoMatchingSchema
	}
	return impl.ValidateRequest(req, v.cfg)
}

// validateResponseWith dispatches to the format-specific validator for a response.
func (v *Validator) validateResponseWith(resp *http.Response, req *http.Request, s Schema) (*ValidationResult, error) {
	impl, ok := schemaImpl(s)
	if !ok {
		return nil, ErrNoMatchingSchema
	}
	return impl.ValidateResponse(resp, req, v.cfg)
}

// noSchemaResult returns the appropriate outcome when no schema matches.
// schemaFound indicates whether a schema matched the transport but had no route
// for this path (true), vs. no schema being registered for this transport at
// all (false). The distinction only affects the StrictMode violation message.
// With IgnoreUnknown: silently pass. With StrictMode: return a violation.
// Default: return ErrNoMatchingSchema.
func (v *Validator) noSchemaResult(schemaFound bool) (*ValidationResult, error) {
	if v.cfg.IgnoreUnknown {
		return &ValidationResult{Valid: true}, nil
	}
	if v.cfg.StrictMode {
		msg := "no registered schema covers this endpoint"
		if schemaFound {
			msg = "endpoint not found in any registered schema"
		}
		return &ValidationResult{
			Valid: false,
			Violations: []Violation{{
				Code:     CodeUnknownEndpoint,
				Severity: SeverityError,
				Message:  msg,
			}},
		}, nil
	}
	return nil, ErrNoMatchingSchema
}
