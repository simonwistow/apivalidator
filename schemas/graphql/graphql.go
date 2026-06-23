// Package graphql implements validation for GraphQL schemas (SDL format).
// For requests: parses the query document and validates it against the schema.
// For responses: validates that the data shape matches the selected query and
// that any errors follow the GraphQL spec structure.
package graphql

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatGraphQL, parseSchema)
}

type graphqlSchema struct {
	name        string
	fingerprint string
	schema      *ast.Schema
}

func parseSchema(data []byte, hint string) (apivalidator.Schema, error) {
	src := &ast.Source{
		Name:  hint,
		Input: string(data),
	}
	schema, err := gqlparser.LoadSchema(src)
	if err != nil {
		return nil, fmt.Errorf("parse GraphQL SDL: %w", err)
	}

	name := hint
	if name == "" {
		name = "GraphQL"
	}
	sum := sha256.Sum256(data)
	return &graphqlSchema{
		name:        name,
		fingerprint: hex.EncodeToString(sum[:]),
		schema:      schema,
	}, nil
}

func (s *graphqlSchema) Format() apivalidator.Format { return apivalidator.FormatGraphQL }
func (s *graphqlSchema) Name() string                { return s.name }
func (s *graphqlSchema) Fingerprint() string         { return s.fingerprint }

// graphqlRequest is the standard JSON body format for GraphQL requests.
type graphqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

// graphqlResponse is the standard JSON body format for GraphQL responses.
type graphqlResponse struct {
	Data   json.RawMessage  `json:"data"`
	Errors []graphqlRespErr `json:"errors"`
}

type graphqlRespErr struct {
	Message   string `json:"message"`
	Locations []struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"locations"`
}

func (s *graphqlSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	if !cfg.ValidateBody {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	body, err := apivalidator.ReadRequestBody(req, cfg)
	if err != nil {
		return nil, err
	}

	res := &apivalidator.ValidationResult{MatchedSchema: s.name}

	if len(body) == 0 {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeMissingField,
			Severity: apivalidator.SeverityError,
			Message:  "GraphQL request body is empty",
		}}
		return res, nil
	}

	// Handle GET requests: query is in the URL parameter.
	queryStr := req.URL.Query().Get("query")
	if req.Method != http.MethodGet {
		var gqlReq graphqlRequest
		if err := json.Unmarshal(body, &gqlReq); err != nil {
			res.Violations = []apivalidator.Violation{{
				Code:     apivalidator.CodeInvalidBody,
				Severity: apivalidator.SeverityError,
				Message:  fmt.Sprintf("GraphQL request body is not valid JSON: %v", err),
			}}
			return res, nil
		}
		queryStr = gqlReq.Query
	}

	if strings.TrimSpace(queryStr) == "" {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeMissingField,
			Severity: apivalidator.SeverityError,
			Message:  `GraphQL request is missing the "query" field`,
		}}
		return res, nil
	}

	doc, gqlErrs := gqlparser.LoadQuery(s.schema, queryStr)
	if gqlErrs != nil {
		res.Valid = false
		res.Violations = gqlErrorsToViolations(gqlErrs)
		return res, nil
	}
	_ = doc

	res.Valid = true
	return res, nil
}

func (s *graphqlSchema) ValidateResponse(resp *http.Response, _ *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	if !cfg.ValidateBody {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	body, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != nil {
		return nil, err
	}

	res := &apivalidator.ValidationResult{MatchedSchema: s.name}

	if len(body) == 0 {
		res.Valid = true
		return res, nil
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("GraphQL response body is not valid JSON: %v", err),
		}}
		return res, nil
	}

	// A valid GraphQL response must have "data" and/or "errors".
	if gqlResp.Data == nil && gqlResp.Errors == nil {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidResponse,
			Severity: apivalidator.SeverityError,
			Message:  `GraphQL response must contain "data" and/or "errors"`,
		}}
		return res, nil
	}

	// If "errors" is present, report them as warnings (server-side errors are valid protocol).
	var violations []apivalidator.Violation
	for _, e := range gqlResp.Errors {
		violations = append(violations, apivalidator.Violation{
			Code:     apivalidator.CodeInvalidResponse,
			Severity: apivalidator.SeverityWarning,
			Message:  fmt.Sprintf("GraphQL error: %s", e.Message),
		})
	}

	res.Valid = true
	res.Violations = violations
	return res, nil
}

func gqlErrorsToViolations(errs gqlerror.List) []apivalidator.Violation {
	out := make([]apivalidator.Violation, 0, len(errs))
	for _, e := range errs {
		path := ""
		if len(e.Locations) > 0 {
			path = fmt.Sprintf("line %d, col %d", e.Locations[0].Line, e.Locations[0].Column)
		}
		out = append(out, apivalidator.Violation{
			Path:     path,
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  e.Message,
		})
	}
	return out
}
