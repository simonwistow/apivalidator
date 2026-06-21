package raml_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/raml"
)

// testRAML is a small but representative RAML 1.0 API used across tests.
const testRAML = `#%RAML 1.0
title: Test API
mediaType: application/json

/users:
  get:
    queryParameters:
      filter:
        type: string
        required: true
      page:
        type: integer
        required: false
    responses:
      200:
  post:
    body:
      application/json:
        type: object
        properties:
          name:
            type: string
            required: true
          age:
            type: integer
    responses:
      201:

/users/{userId}:
  get:
    responses:
      200:
      404:
  delete:
    responses:
      204:
`

func mustParseRAML(t *testing.T) apivalidator.Schema {
	t.Helper()
	s, err := apivalidator.ParseSchemaString(apivalidator.FormatRAML, testRAML)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	return s
}

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	return apivalidator.NewValidator().AddSchema(mustParseRAML(t))
}

func getReq(path, query string) *http.Request {
	url := path
	if query != "" {
		url = path + "?" + query
	}
	return httptest.NewRequest(http.MethodGet, url, nil)
}

func postReq(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path,
		io.NopCloser(strings.NewReader(body)))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func delReq(path string) *http.Request {
	return httptest.NewRequest(http.MethodDelete, path, nil)
}

func responseFor(req *http.Request, statusCode int, body string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Request:    req,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	if body != "" {
		resp.Body = io.NopCloser(strings.NewReader(body))
	} else {
		resp.Body = http.NoBody
	}
	return resp
}

// ── Parse ──────────────────────────────────────────────────────────────────────

func TestRAML_ParseSuccess(t *testing.T) {
	s := mustParseRAML(t)
	if s.Format() != apivalidator.FormatRAML {
		t.Errorf("Format = %v, want FormatRAML", s.Format())
	}
	if s.Name() != "Test API" {
		t.Errorf("Name = %q, want %q", s.Name(), "Test API")
	}
	if s.Fingerprint() == "" {
		t.Error("Fingerprint is empty")
	}
}

func TestRAML_ParseInvalid(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatRAML, "not raml at all")
	if err == nil {
		t.Fatal("expected parse error for invalid RAML")
	}
}

// ── Request: path routing ──────────────────────────────────────────────────────

func TestRAML_Request_UnknownPath(t *testing.T) {
	v := newValidator(t)
	_, err := v.ValidateRequest(getReq("/unknown", ""))
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema, got %v", err)
	}
}

func TestRAML_Request_UnknownMethod(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPut, "/users/123", nil)
	result, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid result for undefined method")
	}
	if len(result.Violations) == 0 || result.Violations[0].Code != apivalidator.CodeUnknownMethod {
		t.Errorf("expected CodeUnknownMethod, got %v", result.Violations)
	}
}

func TestRAML_Request_PathParamMatching(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(getReq("/users/abc-123", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid for path-param request: %v", result.Violations)
	}
	if result.MatchedEndpoint != "/users/{userId}" {
		t.Errorf("MatchedEndpoint = %q, want /users/{userId}", result.MatchedEndpoint)
	}
}

// ── Request: query parameters ─────────────────────────────────────────────────

func TestRAML_Request_RequiredQueryParam_Present(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(getReq("/users", "filter=active"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid when required query param present: %v", result.Violations)
	}
}

func TestRAML_Request_RequiredQueryParam_Missing(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(getReq("/users", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid when required query param missing")
	}
	if len(result.Violations) == 0 || result.Violations[0].Code != apivalidator.CodeMissingQueryParam {
		t.Errorf("expected CodeMissingQueryParam, got %v", result.Violations)
	}
}

func TestRAML_Request_RequiredQueryParam_ExplicitEmpty(t *testing.T) {
	// ?filter= supplies the key with an empty value — that counts as present,
	// so CodeMissingQueryParam must not fire.
	v := newValidator(t)
	result, err := v.ValidateRequest(getReq("/users", "filter="))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid for explicitly-empty required param: %v", result.Violations)
	}
}

func TestRAML_Request_OptionalQueryParam_Absent(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(getReq("/users", "filter=active"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid without optional param: %v", result.Violations)
	}
}

func TestRAML_Request_QueryParamValidation_Disabled(t *testing.T) {
	s := mustParseRAML(t)
	v := apivalidator.NewValidator(apivalidator.WithValidateQueryParams(false)).AddSchema(s)
	result, err := v.ValidateRequest(getReq("/users", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid when query param validation disabled: %v", result.Violations)
	}
}

// ── Request: body validation ───────────────────────────────────────────────────

func TestRAML_Request_ValidBody(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(postReq("/users", `{"name":"Alice","age":30}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid body: %v", result.Violations)
	}
}

func TestRAML_Request_InvalidBody_MissingRequired(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(postReq("/users", `{"age":30}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid when required body field missing")
	}
	if len(result.Violations) == 0 || result.Violations[0].Code != apivalidator.CodeInvalidBody {
		t.Errorf("expected CodeInvalidBody, got %v", result.Violations)
	}
}

func TestRAML_Request_InvalidBody_BadJSON(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(postReq("/users", `not json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for bad JSON body")
	}
}

func TestRAML_Request_NoBody_Skipped(t *testing.T) {
	v := newValidator(t)
	// POST /users with no body — body validation should be skipped
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	result, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid for missing body: %v", result.Violations)
	}
}

func TestRAML_Request_BodyValidation_Disabled(t *testing.T) {
	s := mustParseRAML(t)
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false)).AddSchema(s)
	result, err := v.ValidateRequest(postReq("/users", `{"age":30}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid when body validation disabled: %v", result.Violations)
	}
}

// ── DELETE (no query params, no body) ─────────────────────────────────────────

func TestRAML_Request_Delete(t *testing.T) {
	v := newValidator(t)
	result, err := v.ValidateRequest(delReq("/users/456"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid DELETE: %v", result.Violations)
	}
}

// ── Response validation ────────────────────────────────────────────────────────

func TestRAML_Response_KnownStatusCode(t *testing.T) {
	v := newValidator(t)
	req := getReq("/users/123", "")
	result, err := v.ValidateResponse(responseFor(req, 200, `{"id":"123","name":"Alice"}`), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid 200 response: %v", result.Violations)
	}
}

func TestRAML_Response_UnknownStatusCode(t *testing.T) {
	v := newValidator(t)
	req := getReq("/users/123", "")
	result, err := v.ValidateResponse(responseFor(req, 500, `{"error":"internal"}`), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for undefined status code")
	}
	if len(result.Violations) == 0 || result.Violations[0].Code != apivalidator.CodeUnknownStatusCode {
		t.Errorf("expected CodeUnknownStatusCode, got %v", result.Violations)
	}
}

func TestRAML_Response_404_NoBody(t *testing.T) {
	v := newValidator(t)
	req := getReq("/users/123", "")
	result, err := v.ValidateResponse(responseFor(req, 404, ""), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid 404 with no body: %v", result.Violations)
	}
}

func TestRAML_Response_UnknownPath(t *testing.T) {
	v := newValidator(t)
	req := getReq("/noexist", "")
	_, err := v.ValidateResponse(responseFor(req, 200, ""), req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema, got %v", err)
	}
}

func TestRAML_Response_UnknownMethod(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPut, "/users/123", nil)
	result, err := v.ValidateResponse(responseFor(req, 200, ""), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for undefined method")
	}
	if len(result.Violations) == 0 || result.Violations[0].Code != apivalidator.CodeUnknownMethod {
		t.Errorf("expected CodeUnknownMethod, got %v", result.Violations)
	}
}

// ── Schema metadata ────────────────────────────────────────────────────────────

func TestRAML_FingerprintStable(t *testing.T) {
	s1 := mustParseRAML(t)
	s2 := mustParseRAML(t)
	if s1.Fingerprint() != s2.Fingerprint() {
		t.Error("fingerprint is not stable across parses")
	}
}

func TestRAML_FingerprintDiffers(t *testing.T) {
	s1 := mustParseRAML(t)
	other := `#%RAML 1.0
title: Other API
mediaType: application/json

/other:
  get:
    responses:
      200:
`
	s2, err := apivalidator.ParseSchemaString(apivalidator.FormatRAML, other)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s1.Fingerprint() == s2.Fingerprint() {
		t.Error("different schemas should have different fingerprints")
	}
}
