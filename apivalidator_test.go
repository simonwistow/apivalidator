package apivalidator_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/schemas/openapi"
	apivalidator "github.com/simonwistow/apivalidator"
)

// minimalOpenAPI is a tiny OpenAPI 3.x document with one endpoint.
const minimalOpenAPI = `{
  "openapi": "3.0.0",
  "info": {"title": "Test API", "version": "1.0.0"},
  "paths": {
    "/users/{userID}": {
      "get": {
        "operationId": "getUser",
        "parameters": [
          {"name": "userID", "in": "path", "required": true, "schema": {"type": "integer"}},
          {"name": "include", "in": "query", "required": false, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}, "404": {"description": "Not found"}}
      }
    }
  }
}`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(minimalOpenAPI)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

func TestValidateRequest_KnownEndpoint(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got violations: %s", res.Error())
	}
	if res.MatchedEndpoint == "" {
		t.Error("MatchedEndpoint should not be empty")
	}
}

func TestValidateRequest_UnknownEndpoint(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema, got %v", err)
	}
}

func TestValidateRequest_IgnoreUnknown(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithIgnoreUnknownEndpoints())
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(minimalOpenAPI)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for unknown endpoint with IgnoreUnknown, got: %s", res.Error())
	}
}

func TestValidateRequest_UnknownQueryParam(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/users/42?foo=bar", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unknown query params are warnings, not errors — result should still be valid.
	if !res.Valid {
		t.Errorf("expected valid (unknown query param is warning only), got: %s", res.Error())
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeUnknownQueryParam {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeUnknownQueryParam violation")
	}
}

func TestValidateResponse_KnownStatus(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	resp := &http.Response{StatusCode: 200, Request: req}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid 200, got: %s", res.Error())
	}
}

func TestValidateResponse_UnknownStatus(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	resp := &http.Response{StatusCode: 500, Request: req}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unknown status codes are warnings — still valid.
	if !res.Valid {
		t.Errorf("expected valid (unknown status is warning only), got: %s", res.Error())
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeUnknownStatusCode {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeUnknownStatusCode violation")
	}
}

func TestOneShotValidateRequest(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatOpenAPI3, minimalOpenAPI)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/users/99", nil)
	res, err := apivalidator.ValidateRequest(req, schema)
	if err != nil {
		t.Fatalf("ValidateRequest: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid: %s", res.Error())
	}
}

func TestMultiSchemaPolicyAll(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAll))
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(minimalOpenAPI)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid: %s", res.Error())
	}
}
