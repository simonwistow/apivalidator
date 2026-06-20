package apivalidator_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/all"
	apivalidator "github.com/simonwistow/apivalidator"
)

// FuzzValidateRequest ensures the validator never panics on arbitrary request bodies.
func FuzzValidateRequest(f *testing.F) {
	// Seed corpus.
	f.Add([]byte(`{"id":1,"name":"Alice"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(``))
	f.Add([]byte(`not json at all`))
	f.Add([]byte{0x00, 0x01, 0xff})

	const schema = `{
	  "type": "object",
	  "required": ["id"],
	  "properties": {
	    "id":   {"type": "integer"},
	    "name": {"type": "string"}
	  }
	}`
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		req := httptest.NewRequest(http.MethodPost, "/anything", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Must not panic; return value and error are unimportant.
		v.ValidateRequest(req) //nolint:errcheck
	})
}

// FuzzParseSchema ensures schema parsing never panics on arbitrary input.
func FuzzParseSchema(f *testing.F) {
	f.Add([]byte(`{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic regardless of input.
		apivalidator.ParseSchema(apivalidator.FormatOpenAPI3, data)  //nolint:errcheck
		apivalidator.ParseSchema(apivalidator.FormatJSONSchema, data) //nolint:errcheck
	})
}

// BenchmarkValidateRequest measures throughput for OpenAPI request validation.
func BenchmarkValidateRequest(b *testing.B) {
	const schema = `{
	  "openapi": "3.0.0",
	  "info": {"title":"API","version":"1"},
	  "paths": {
	    "/users/{id}": {
	      "get": {
	        "parameters": [{"name":"id","in":"path","required":true,"schema":{"type":"integer"}}],
	        "responses": {"200":{"description":"OK"}}
	      }
	    }
	  }
	}`
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(schema)); err != nil {
		b.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		v.ValidateRequest(req) //nolint:errcheck
	}
}

// BenchmarkValidateJSONBody measures throughput for JSON Schema body validation.
func BenchmarkValidateJSONBody(b *testing.B) {
	schema := `{"type":"object","required":["id","name"],"properties":{"id":{"type":"integer"},"name":{"type":"string"}}}`
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		b.Fatal(err)
	}
	bodyBytes := []byte(`{"id":1,"name":"Alice"}`)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateRequest(req) //nolint:errcheck
	}
}

// Ensure the http package import is used.
var _ = http.MethodGet
