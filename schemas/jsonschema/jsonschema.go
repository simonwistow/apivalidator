// Package jsonschema implements body validation using JSON Schema documents.
// Supports drafts 07, 2019-09, and 2020-12 (auto-detected from $schema keyword).
//
// When registered as a standalone schema (not embedded in an OpenAPI document),
// it applies to both request and response bodies by default. Control scope with
// WithJSONSchemaScope.
package jsonschema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/santhosh-tekuri/jsonschema/v5"

	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatJSONSchema, parseSchema)
}

type jsonSchemaSchema struct {
	name        string
	fingerprint string
	compiled    *jsonschema.Schema
}

func parseSchema(data []byte, hint string) (apivalidator.Schema, error) {
	compiler := jsonschema.NewCompiler()
	url := "mem:///schema.json"
	if err := compiler.AddResource(url, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	compiled, err := compiler.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	name := extractTitle(data)
	if name == "" {
		name = hint
	}
	if name == "" {
		name = "JSON Schema"
	}

	sum := sha256.Sum256(data)
	return &jsonSchemaSchema{
		name:        name,
		fingerprint: hex.EncodeToString(sum[:]),
		compiled:    compiled,
	}, nil
}

func extractTitle(data []byte) string {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	t, _ := doc["title"].(string)
	return t
}

func (s *jsonSchemaSchema) Format() apivalidator.Format { return apivalidator.FormatJSONSchema }
func (s *jsonSchemaSchema) Name() string                { return s.name }
func (s *jsonSchemaSchema) Fingerprint() string         { return s.fingerprint }

func (s *jsonSchemaSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	if cfg.JSONSchemaScope == apivalidator.JSONSchemaResponse {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	if !cfg.ValidateBody {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	body, err := apivalidator.ReadRequestBody(req, cfg)
	if err != nil {
		return nil, err
	}
	return s.validateBody(body), nil
}

func (s *jsonSchemaSchema) ValidateResponse(resp *http.Response, _ *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	if cfg.JSONSchemaScope == apivalidator.JSONSchemaRequest {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	if !cfg.ValidateBody {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	body, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != nil {
		return nil, err
	}
	return s.validateBody(body), nil
}

// ValidateBody validates raw JSON bytes directly. Exported so the openapi
// package can call it for per-operation body schemas.
func (s *jsonSchemaSchema) ValidateBody(body []byte) *apivalidator.ValidationResult {
	return s.validateBody(body)
}

func (s *jsonSchemaSchema) validateBody(body []byte) *apivalidator.ValidationResult {
	res := &apivalidator.ValidationResult{MatchedSchema: s.name}

	if len(body) == 0 {
		res.Valid = true
		return res
	}

	var instance any
	if err := json.Unmarshal(body, &instance); err != nil {
		res.Valid = false
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("body is not valid JSON: %v", err),
		}}
		return res
	}

	if err := s.compiled.Validate(instance); err != nil {
		res.Valid = false
		res.Violations = collectErrors(err)
		return res
	}

	res.Valid = true
	return res
}

func collectErrors(err error) []apivalidator.Violation {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  err.Error(),
		}}
	}
	var out []apivalidator.Violation
	flattenErrors(ve, &out)
	return out
}

func flattenErrors(e *jsonschema.ValidationError, out *[]apivalidator.Violation) {
	if len(e.Causes) == 0 {
		*out = append(*out, apivalidator.Violation{
			Path:     e.InstanceLocation,
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  e.Message,
		})
		return
	}
	for _, c := range e.Causes {
		flattenErrors(c, out)
	}
}
