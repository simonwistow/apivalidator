// Package raml implements RAML 1.0 validation using acronis/go-raml.
// Import this package for its side effect of registering the RAML parser.
//
// Usage:
//
//	import _ "github.com/simonwistow/apivalidator/schemas/raml"
package raml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	goraml "github.com/acronis/go-raml/v3"

	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatRAML, parseSchemaBytes)
	apivalidator.RegisterFileParser(apivalidator.FormatRAML, parseSchemaFromPath)
}

type ramlSchema struct {
	name        string
	fingerprint string
	endpoints   []*ramlEndpoint
}

type ramlEndpoint struct {
	pattern *regexp.Regexp
	params  []string
	method  string
	op      *goraml.Operation
	fullURI string
}

func (s *ramlSchema) Format() apivalidator.Format { return apivalidator.FormatRAML }
func (s *ramlSchema) Name() string                { return s.name }
func (s *ramlSchema) Fingerprint() string         { return s.fingerprint }

func parseSchemaBytes(data []byte, hint string) (apivalidator.Schema, error) {
	name := hint
	if name == "" {
		name = "api.raml"
	}
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "/"
	}
	r, err := goraml.ParseFromString(string(data), name, baseDir, goraml.OptWithUnwrap())
	if err != nil {
		return nil, fmt.Errorf("parse RAML: %w", err)
	}
	return buildSchema(r, data, name)
}

func parseSchemaFromPath(path string) (apivalidator.Schema, error) {
	r, err := goraml.ParseFromPath(path, goraml.OptWithUnwrap())
	if err != nil {
		return nil, fmt.Errorf("parse RAML: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s for fingerprint: %w", path, err)
	}
	return buildSchema(r, data, filepath.Base(path))
}

func buildSchema(r *goraml.RAML, data []byte, hint string) (apivalidator.Schema, error) {
	frag := r.EntryPoint()
	api, ok := frag.(*goraml.APIFragment)
	if !ok {
		return nil, fmt.Errorf("RAML document is not an API root (got %T)", frag)
	}

	s := &ramlSchema{}
	if api.Title != nil {
		s.name = api.Title.Value
	}
	if s.name == "" {
		s.name = hint
	}

	sum := sha256.Sum256(data)
	s.fingerprint = hex.EncodeToString(sum[:])

	if api.EndPoints != nil {
		for pair := api.EndPoints.Oldest(); pair != nil; pair = pair.Next() {
			s.walkEndpoint(pair.Value)
		}
	}

	return s, nil
}

func (s *ramlSchema) walkEndpoint(ep *goraml.EndPoint) {
	if ep.Operations != nil {
		for opPair := ep.Operations.Oldest(); opPair != nil; opPair = opPair.Next() {
			s.addOperation(ep, opPair.Value)
		}
	}
	if ep.EndPoints != nil {
		for subPair := ep.EndPoints.Oldest(); subPair != nil; subPair = subPair.Next() {
			s.walkEndpoint(subPair.Value)
		}
	}
}

func (s *ramlSchema) addOperation(ep *goraml.EndPoint, op *goraml.Operation) {
	pattern, params, err := compileURITemplate(ep.FullURI)
	if err != nil {
		return
	}
	s.endpoints = append(s.endpoints, &ramlEndpoint{
		pattern: pattern,
		params:  params,
		method:  strings.ToUpper(op.Method),
		op:      op,
		fullURI: ep.FullURI,
	})
}

// uriTemplateRe matches a single {paramName} placeholder.
var uriTemplateRe = regexp.MustCompile(`\{([^}]+)\}`)

// compileURITemplate converts a RAML URI template like /users/{userId}/posts
// into a regexp and an ordered list of parameter names. Literal path segments
// are passed through regexp.QuoteMeta so dots, plus signs, and other regex
// metacharacters are matched literally.
func compileURITemplate(uri string) (*regexp.Regexp, []string, error) {
	var params []string
	var b strings.Builder
	b.WriteString("^")
	lastEnd := 0
	for _, loc := range uriTemplateRe.FindAllStringIndex(uri, -1) {
		// Literal segment before this {param}.
		b.WriteString(regexp.QuoteMeta(uri[lastEnd:loc[0]]))
		// Capture group for the parameter value.
		params = append(params, uri[loc[0]+1:loc[1]-1])
		b.WriteString(`([^/]+)`)
		lastEnd = loc[1]
	}
	// Any trailing literal segment after the last {param}.
	b.WriteString(regexp.QuoteMeta(uri[lastEnd:]))
	b.WriteString("$")

	compiled, err := regexp.Compile(b.String())
	if err != nil {
		return nil, nil, err
	}
	return compiled, params, nil
}

// findEndpoint resolves a path and method to a matching endpoint.
// pathMatch is the first endpoint whose URI pattern matches path (non-nil
// when the path is known but the method is not); ep is the endpoint with
// both a matching path and method (nil when the method is not defined).
func (s *ramlSchema) findEndpoint(path, method string) (ep *ramlEndpoint, pathMatch *ramlEndpoint) {
	for _, e := range s.endpoints {
		if e.pattern.MatchString(path) {
			if pathMatch == nil {
				pathMatch = e
			}
			if e.method == method {
				ep = e
				return
			}
		}
	}
	return
}

func (s *ramlSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	method := strings.ToUpper(req.Method)
	ep, pathMatch := s.findEndpoint(req.URL.Path, method)
	if pathMatch == nil {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	if ep == nil {
		return &apivalidator.ValidationResult{
			Valid:           false,
			MatchedSchema:   s.name,
			MatchedEndpoint: pathMatch.fullURI,
			Violations: []apivalidator.Violation{{
				Code:     apivalidator.CodeUnknownMethod,
				Severity: apivalidator.SeverityError,
				Message:  fmt.Sprintf("method %s is not defined for this endpoint", method),
			}},
		}, nil
	}

	result := &apivalidator.ValidationResult{
		Valid:           true,
		MatchedSchema:   s.name,
		MatchedEndpoint: ep.fullURI,
	}

	if cfg.ValidateQueryParams {
		viols := queryParamViolations(req, ep.op)
		result.Violations = append(result.Violations, viols...)
	}

	if cfg.ValidateBody {
		body, err := apivalidator.ReadRequestBody(req, cfg)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 && ep.op.Request != nil {
			viols, err := requestBodyViolations(body, ep.op.Request)
			if err != nil {
				return nil, err
			}
			result.Violations = append(result.Violations, viols...)
		}
	}

	result.Valid = len(result.Violations) == 0
	return result, nil
}

func (s *ramlSchema) ValidateResponse(resp *http.Response, req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	method := strings.ToUpper(req.Method)
	ep, pathMatch := s.findEndpoint(req.URL.Path, method)
	if pathMatch == nil {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	if ep == nil {
		return &apivalidator.ValidationResult{
			Valid:           false,
			MatchedSchema:   s.name,
			MatchedEndpoint: pathMatch.fullURI,
			Violations: []apivalidator.Violation{{
				Code:     apivalidator.CodeUnknownMethod,
				Severity: apivalidator.SeverityError,
				Message:  fmt.Sprintf("method %s is not defined for this endpoint", method),
			}},
		}, nil
	}

	result := &apivalidator.ValidationResult{
		Valid:           true,
		MatchedSchema:   s.name,
		MatchedEndpoint: ep.fullURI,
	}

	if ep.op.Responses == nil {
		return result, nil
	}

	respDef, ok := ep.op.Responses.Get(resp.StatusCode)
	if !ok {
		result.Valid = false
		result.Violations = append(result.Violations, apivalidator.Violation{
			Code:     apivalidator.CodeUnknownStatusCode,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("status code %d is not defined for this endpoint", resp.StatusCode),
		})
		return result, nil
	}

	if cfg.ValidateBody {
		body, err := apivalidator.ReadResponseBody(resp, cfg)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			viols, err := responseBodyViolations(body, respDef)
			if err != nil {
				return nil, err
			}
			result.Violations = append(result.Violations, viols...)
		}
	}

	result.Valid = len(result.Violations) == 0
	return result, nil
}

// queryParamViolations checks required query parameters against the request URL.
// A parameter is considered missing only when its key is entirely absent from
// the query string; an explicit empty value (?param=) is treated as present.
func queryParamViolations(req *http.Request, op *goraml.Operation) []apivalidator.Violation {
	if op.QueryParameters == nil {
		return nil
	}
	query := req.URL.Query()
	var viols []apivalidator.Violation
	for pair := op.QueryParameters.Oldest(); pair != nil; pair = pair.Next() {
		prop := pair.Value
		if prop.Required {
			if _, ok := query[prop.Name]; !ok {
				viols = append(viols, apivalidator.Violation{
					Code:     apivalidator.CodeMissingQueryParam,
					Path:     prop.Name,
					Severity: apivalidator.SeverityError,
					Message:  fmt.Sprintf("required query parameter %q is missing", prop.Name),
				})
			}
		}
	}
	return viols
}

func requestBodyViolations(body []byte, req *goraml.Request) ([]apivalidator.Violation, error) {
	if req.Bodies == nil {
		return nil, nil
	}
	var bodyDef *goraml.Body
	if b, ok := req.Bodies.Get("application/json"); ok {
		bodyDef = b
	} else if pair := req.Bodies.Oldest(); pair != nil {
		bodyDef = pair.Value
	}
	return validateBodyShape(body, bodyDef)
}

func responseBodyViolations(body []byte, resp *goraml.Response) ([]apivalidator.Violation, error) {
	if resp.Bodies == nil {
		return nil, nil
	}
	var bodyDef *goraml.Body
	if b, ok := resp.Bodies.Get("application/json"); ok {
		bodyDef = b
	} else if pair := resp.Bodies.Oldest(); pair != nil {
		bodyDef = pair.Value
	}
	return validateBodyShape(body, bodyDef)
}

// validateBodyShape unmarshals the JSON body and validates it against the RAML
// BaseShape. Returns nil violations when the body conforms.
func validateBodyShape(body []byte, bodyDef *goraml.Body) ([]apivalidator.Violation, error) {
	if bodyDef == nil || bodyDef.Shape == nil {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  "body is not valid JSON: " + err.Error(),
		}}, nil
	}
	if err := bodyDef.Shape.Validate(v); err != nil {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  err.Error(),
		}}, nil
	}
	return nil, nil
}
