package jsonschema_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
)

func TestJSONSchema_Name_Fingerprint_Format(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatJSONSchema,
		`{"title":"MyAPI","type":"object"}`)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() != "MyAPI" {
		t.Errorf("expected Name=MyAPI, got %q", schema.Name())
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
	if schema.Format() != apivalidator.FormatJSONSchema {
		t.Errorf("expected FormatJSONSchema, got %v", schema.Format())
	}
}

func TestJSONSchema_Name_FallsBackToHint(t *testing.T) {
	// No title in schema — Name should fall back to the hint (empty string → "JSON Schema").
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatJSONSchema, `{"type":"object"}`)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name when no title (defaults to 'JSON Schema')")
	}
}

func TestJSONSchema_ParseInvalid(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatJSONSchema, `{not valid json`)
	if err == nil {
		t.Error("expected error for invalid JSON Schema")
	}
}

func TestJSONSchema_ValidateResponse_Valid(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"id":1,"name":"Bob"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid response, got: %s", res.Error())
	}
}

func TestJSONSchema_ValidateResponse_Invalid(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"id":"not-int","name":"Bob"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid response body (wrong type for id)")
	}
}

func TestJSONSchema_ValidateResponse_EmptyBody(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("")),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for empty response body, got: %s", res.Error())
	}
}

func TestJSONSchema_ValidateResponse_NilBody(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       nil,
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for nil response body, got: %s", res.Error())
	}
}

func TestJSONSchema_ValidateResponse_InvalidJSON(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{not json`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for non-JSON response body")
	}
}

func TestJSONSchema_ValidateResponse_ScopeRequestOnly(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaRequest))
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	// Invalid response body — but scope is request-only, so response is skipped.
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"wrong":"field"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with request-only scope (response skipped), got: %s", res.Error())
	}
}

func TestJSONSchema_ValidateResponse_ValidateBodyFalse(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"wrong":"field"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with body validation off, got: %s", res.Error())
	}
}

func TestJSONSchema_ValidateBody_AdditionalProperties(t *testing.T) {
	v := apivalidator.NewValidator()
	// additionalProperties: false is already in userSchema.
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	req := postJSON(`{"id":1,"name":"Alice","extra":"field"}`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for extra field with additionalProperties:false")
	}
}

func TestJSONSchema_PackageLevelValidateResponse(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatJSONSchema,
		`{"title":"T","type":"object","required":["ok"],"properties":{"ok":{"type":"boolean"}}}`)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
		Request:    req,
	}
	res, err := apivalidator.ValidateResponse(resp, req, schema)
	if err != nil {
		t.Fatalf("ValidateResponse: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid: %s", res.Error())
	}
}
