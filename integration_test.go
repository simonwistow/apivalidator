package apivalidator_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/all"
	apivalidator "github.com/simonwistow/apivalidator"
)

// TestFullPipeline exercises the complete path from schema registration through
// request validation and response validation using the OpenAPI schema.
func TestFullPipeline(t *testing.T) {
	const schema = `{
	  "openapi": "3.0.0",
	  "info": {"title": "Pet Store", "version": "1.0.0"},
	  "paths": {
	    "/pets/{petID}": {
	      "get": {
	        "operationId": "getPet",
	        "parameters": [
	          {"name": "petID", "in": "path", "required": true, "schema": {"type": "integer"}}
	        ],
	        "responses": {
	          "200": {"description": "OK"},
	          "404": {"description": "Not found"}
	        }
	      }
	    },
	    "/pets": {
	      "post": {
	        "operationId": "createPet",
	        "responses": {"201": {"description": "Created"}}
	      }
	    }
	  }
	}`

	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(schema)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}

	// Known GET with valid path param.
	req := httptest.NewRequest(http.MethodGet, "/pets/42", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("GET /pets/42: %v", err)
	}
	if !res.Valid {
		t.Errorf("GET /pets/42: expected valid, got: %s", res.Error())
	}
	if res.MatchedEndpoint == "" {
		t.Error("GET /pets/42: MatchedEndpoint should not be empty")
	}

	// POST to known endpoint.
	body := bytes.NewBufferString(`{"name":"Fluffy"}`)
	req = httptest.NewRequest(http.MethodPost, "/pets", body)
	req.Header.Set("Content-Type", "application/json")
	res, err = v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("POST /pets: %v", err)
	}
	if !res.Valid {
		t.Errorf("POST /pets: expected valid, got: %s", res.Error())
	}

	// Unknown endpoint.
	req = httptest.NewRequest(http.MethodDelete, "/unknown", nil)
	_, err = v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("DELETE /unknown: expected ErrNoMatchingSchema, got %v", err)
	}

	// Response validation.
	req = httptest.NewRequest(http.MethodGet, "/pets/99", nil)
	resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: http.NoBody, Request: req}
	res, err = v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("response validation: %v", err)
	}
	if !res.Valid {
		t.Errorf("response 200: expected valid, got: %s", res.Error())
	}
}

// TestJSONSchemaBodyValidation verifies that JSON Schema body validation
// integrates correctly with the Validator dispatch layer.
func TestJSONSchemaBodyValidation(t *testing.T) {
	schema := `{
	  "type": "object",
	  "required": ["email"],
	  "properties": {
	    "email": {"type": "string"}
	  }
	}`
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}

	// Valid body.
	body, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/anything", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got: %s", res.Error())
	}

	// Missing required field.
	body, _ = json.Marshal(map[string]string{"name": "Alice"})
	req = httptest.NewRequest(http.MethodPost, "/anything", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err = v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for missing 'email' field")
	}
}

// TestMultiPolicyAny verifies that PolicyAny passes when at least one schema matches.
func TestMultiPolicyAny(t *testing.T) {
	// Two JSON Schemas for the same endpoint — first requires "a", second requires "b".
	schemaA := `{"type":"object","required":["a"],"properties":{"a":{"type":"string"}}}`
	schemaB := `{"type":"object","required":["b"],"properties":{"b":{"type":"string"}}}`

	v := apivalidator.NewValidator(apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAny))
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schemaA)); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schemaB)); err != nil {
		t.Fatal(err)
	}

	// Body satisfies schemaA but not schemaB — PolicyAny should pass.
	body, _ := json.Marshal(map[string]string{"a": "value"})
	req := httptest.NewRequest(http.MethodPost, "/anything", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("PolicyAny: expected valid when one schema passes, got: %s", res.Error())
	}
}

// TestMultiPolicyAll verifies that PolicyAll fails when any schema fails.
func TestMultiPolicyAll(t *testing.T) {
	schemaA := `{"type":"object","required":["a"],"properties":{"a":{"type":"string"}}}`
	schemaB := `{"type":"object","required":["b"],"properties":{"b":{"type":"string"}}}`

	v := apivalidator.NewValidator(apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAll))
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schemaA)); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schemaB)); err != nil {
		t.Fatal(err)
	}

	// Body satisfies schemaA but not schemaB — PolicyAll should fail.
	body, _ := json.Marshal(map[string]string{"a": "value"})
	req := httptest.NewRequest(http.MethodPost, "/anything", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("PolicyAll: expected invalid when one schema fails")
	}
}
