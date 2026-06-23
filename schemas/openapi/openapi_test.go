package openapi_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/openapi"
)

const petStoreSchema = `{
  "openapi": "3.0.0",
  "info": {"title": "Pet Store", "version": "1.0.0"},
  "paths": {
    "/pets/{petId}": {
      "get": {
        "parameters": [
          {"name": "petId", "in": "path", "required": true, "schema": {"type": "integer"}},
          {"name": "include", "in": "query", "required": false, "schema": {"type": "string"}},
          {"name": "lang", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {"description": "OK"},
          "404": {"description": "Not found"}
        }
      }
    },
    "/pets": {
      "get": {
        "parameters": [],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(petStoreSchema)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

func TestOpenAPI_ParseInvalid(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatOpenAPI3, `{not valid json`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestOpenAPI_ParseFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.json")
	if err := os.WriteFile(path, []byte(petStoreSchema), 0644); err != nil {
		t.Fatal(err)
	}
	schema, err := apivalidator.ParseSchemaFile(apivalidator.FormatOpenAPI3, path)
	if err != nil {
		t.Fatalf("ParseSchemaFile: %v", err)
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name from file hint")
	}
}

func TestOpenAPI_SchemaMetadata(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatOpenAPI3, petStoreSchema)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Format() != apivalidator.FormatOpenAPI3 {
		t.Errorf("expected FormatOpenAPI3, got %v", schema.Format())
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name")
	}
}

func TestOpenAPI_ValidRequest(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/pets/42?lang=en", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got: %s", res.Error())
	}
	if res.MatchedEndpoint == "" {
		t.Error("MatchedEndpoint should be set")
	}
}

func TestOpenAPI_InvalidPathParam(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/pets/not-an-int?lang=en", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for non-integer path param")
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeInvalidPathParam {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeInvalidPathParam violation")
	}
}

func TestOpenAPI_RequiredQueryParam_Present(t *testing.T) {
	v := newValidator(t)
	// "lang" is required and provided
	req := httptest.NewRequest(http.MethodGet, "/pets/42?lang=en", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid when required param present, got: %s", res.Error())
	}
}

func TestOpenAPI_UnknownQueryParam(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/pets/42?lang=en&foo=bar", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unknown query param is a warning — still valid.
	if !res.Valid {
		t.Errorf("expected valid for unknown query param (warning only), got: %s", res.Error())
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

func TestOpenAPI_UnknownEndpoint(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema, got %v", err)
	}
}

func TestOpenAPI_ValidResponse_KnownStatus(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/pets/42", nil)
	resp := &http.Response{StatusCode: 200, Request: req}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid 200 response, got: %s", res.Error())
	}
}

func TestOpenAPI_ValidResponse_UnknownStatus(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/pets/42", nil)
	resp := &http.Response{StatusCode: 503, Request: req}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unknown status is a warning — still valid.
	if !res.Valid {
		t.Errorf("expected valid (unknown status is warning), got: %s", res.Error())
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

func TestOpenAPI_ValidResponse_UnknownEndpoint(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	resp := &http.Response{StatusCode: 200, Request: req}
	_, err := v.ValidateResponse(resp, req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for unknown endpoint in response, got %v", err)
	}
}

func TestOpenAPI_EndpointWithNoResponseCodes(t *testing.T) {
	// /pets has a 200 but let's try a response with a status code listed
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/pets", nil)
	resp := &http.Response{StatusCode: 200, Request: req}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got: %s", res.Error())
	}
}

func TestOpenAPI_ValidateQueryParams_Disabled(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateQueryParams(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(petStoreSchema)); err != nil {
		t.Fatal(err)
	}
	// "lang" is required but query param validation is off
	req := httptest.NewRequest(http.MethodGet, "/pets/42", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with query params disabled, got: %s", res.Error())
	}
}

func TestOpenAPI_PackageLevelValidateResponse(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatOpenAPI3, petStoreSchema)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/pets/42", nil)
	resp := &http.Response{StatusCode: 200, Request: req}
	res, err := apivalidator.ValidateResponse(resp, req, schema)
	if err != nil {
		t.Fatalf("ValidateResponse: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid: %s", res.Error())
	}
}
