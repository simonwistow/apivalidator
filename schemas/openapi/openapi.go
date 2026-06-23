// Package openapi implements REST validation for OpenAPI 3.x (3.0/3.1/3.2)
// and Swagger 2.0 schemas, using apianalyzer/parse for document loading and
// apianalyzer.SpecSet.Match for path routing.
package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	aa "github.com/simonwistow/apianalyzer"
	"github.com/simonwistow/apianalyzer/parse"

	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatOpenAPI3, parseSchema)
	apivalidator.RegisterParser(apivalidator.FormatOpenAPI2, parseSchema)
}

type openapiSchema struct {
	name        string
	fingerprint string
	specs       *aa.SpecSet
}

func parseSchema(data []byte, hint string) (apivalidator.Schema, error) {
	specs, err := parse.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse OpenAPI: %w", err)
	}
	name := hint
	if name == "" {
		name = "OpenAPI"
	}
	sum := sha256.Sum256(data)
	return &openapiSchema{
		name:        name,
		fingerprint: hex.EncodeToString(sum[:]),
		specs:       specs,
	}, nil
}

func (s *openapiSchema) Format() apivalidator.Format { return apivalidator.FormatOpenAPI3 }
func (s *openapiSchema) Name() string                { return s.name }
func (s *openapiSchema) Fingerprint() string         { return s.fingerprint }

func (s *openapiSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	match, ok := s.specs.Match(req.Method, req.URL.Path)
	if !ok {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	res := &apivalidator.ValidationResult{
		MatchedSchema:   s.name,
		MatchedEndpoint: match.Spec.Template(aa.TemplateStyleBrace),
	}

	var violations []apivalidator.Violation

	if cfg.ValidateQueryParams {
		violations = append(violations, checkPathParams(match)...)
		violations = append(violations, checkQueryParams(req, match.Spec)...)
	}

	res.Valid = !hasErrors(violations)
	res.Violations = violations
	return res, nil
}

func (s *openapiSchema) ValidateResponse(resp *http.Response, req *http.Request, _ *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	match, ok := s.specs.Match(req.Method, req.URL.Path)
	if !ok {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	res := &apivalidator.ValidationResult{
		MatchedSchema:   s.name,
		MatchedEndpoint: match.Spec.Template(aa.TemplateStyleBrace),
	}

	var violations []apivalidator.Violation

	codes := match.Spec.StatusCodes()
	if len(codes) > 0 {
		found := false
		for _, c := range codes {
			if c == resp.StatusCode {
				found = true
				break
			}
		}
		if !found {
			violations = append(violations, apivalidator.Violation{
				Code:     apivalidator.CodeUnknownStatusCode,
				Severity: apivalidator.SeverityWarning,
				Message:  fmt.Sprintf("status code %d not declared in schema", resp.StatusCode),
			})
		}
	}

	res.Valid = !hasErrors(violations)
	res.Violations = violations
	return res, nil
}

// checkPathParams validates extracted path variable values against declared types.
func checkPathParams(match *aa.MatchResult) []apivalidator.Violation {
	var out []apivalidator.Violation
	varsByName := make(map[string]aa.Variable, len(match.Spec.Variables()))
	for _, v := range match.Spec.Variables() {
		varsByName[v.Name] = v
	}
	for name, value := range match.Vars {
		v, ok := varsByName[name]
		if !ok {
			continue
		}
		if v.Type == "integer" {
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				out = append(out, apivalidator.Violation{
					Path:     "/path/" + name,
					Code:     apivalidator.CodeInvalidPathParam,
					Severity: apivalidator.SeverityError,
					Message:  fmt.Sprintf("path param %q must be an integer, got %q", name, value),
				})
			}
		}
	}
	return out
}

// checkQueryParams validates declared query parameters against the request URL.
func checkQueryParams(req *http.Request, spec *aa.APISpec) []apivalidator.Violation {
	var out []apivalidator.Violation
	query := req.URL.Query()

	declared := make(map[string]bool)
	for _, p := range spec.QueryParams() {
		declared[p.Name] = true
		if p.Required {
			if _, ok := query[p.Name]; !ok {
				out = append(out, apivalidator.Violation{
					Path:     "/query/" + p.Name,
					Code:     apivalidator.CodeMissingQueryParam,
					Severity: apivalidator.SeverityError,
					Message:  fmt.Sprintf("required query param %q is missing", p.Name),
				})
			}
		}
	}
	for key := range query {
		if !declared[key] {
			out = append(out, apivalidator.Violation{
				Path:     "/query/" + key,
				Code:     apivalidator.CodeUnknownQueryParam,
				Severity: apivalidator.SeverityWarning,
				Message:  fmt.Sprintf("query param %q is not declared in the schema", key),
			})
		}
	}
	return out
}

func hasErrors(vs []apivalidator.Violation) bool {
	for _, v := range vs {
		if v.Severity == apivalidator.SeverityError {
			return true
		}
	}
	return false
}
