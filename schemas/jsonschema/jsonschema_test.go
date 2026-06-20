package jsonschema_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
	apivalidator "github.com/simonwistow/apivalidator"
)

const userSchema = `{
  "title": "User",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["id", "name"],
  "properties": {
    "id":   {"type": "integer"},
    "name": {"type": "string"},
    "email":{"type": "string", "format": "email"}
  },
  "additionalProperties": false
}`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

func postJSON(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/anything", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestJSONSchema_ValidBody(t *testing.T) {
	v := newValidator(t)
	req := postJSON(`{"id":1,"name":"Alice"}`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got: %s", res.Error())
	}
}

func TestJSONSchema_MissingRequired(t *testing.T) {
	v := newValidator(t)
	req := postJSON(`{"id":1}`) // missing "name"
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid (missing required field 'name')")
	}
}

func TestJSONSchema_WrongType(t *testing.T) {
	v := newValidator(t)
	req := postJSON(`{"id":"not-an-int","name":"Alice"}`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid (id should be integer)")
	}
}

func TestJSONSchema_InvalidJSON(t *testing.T) {
	v := newValidator(t)
	req := postJSON(`{not valid json`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for malformed JSON")
	}
}

func TestJSONSchema_EmptyBody(t *testing.T) {
	v := newValidator(t)
	req := postJSON(``)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty body passes — caller's responsibility to require non-empty bodies.
	if !res.Valid {
		t.Errorf("expected valid for empty body, got: %s", res.Error())
	}
}

func TestJSONSchema_ScopeRequestOnly(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaRequest))
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	// POST with invalid body — should be caught.
	req := postJSON(`{"id":"bad"}`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("request scope: expected invalid body to fail")
	}
}

func TestJSONSchema_ScopeResponseOnly(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaResponse))
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(userSchema)); err != nil {
		t.Fatal(err)
	}
	// POST with invalid body — should be skipped (response scope only).
	req := postJSON(`{"id":"bad"}`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Error("response-only scope: request body should be skipped")
	}
}
