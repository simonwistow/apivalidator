package apivalidator_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
)

const testSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"required": ["name"],
	"properties": {
		"name": {"type": "string"}
	}
}`

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func newJSONSchemaValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v, err := apivalidator.NewValidator().AddSchemaString(apivalidator.FormatJSONSchema, testSchema)
	if err != nil {
		t.Fatalf("AddSchemaString: %v", err)
	}
	return v
}

func TestMiddleware_ValidRequest_PassesThrough(t *testing.T) {
	v := newJSONSchemaValidator(t)
	mw := apivalidator.Middleware(v)
	handler := mw(http.HandlerFunc(okHandler))

	body := strings.NewReader(`{"name":"alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Errorf("expected body 'ok', got %q", got)
	}
}

func TestMiddleware_InvalidRequest_Returns400(t *testing.T) {
	v := newJSONSchemaValidator(t)
	mw := apivalidator.Middleware(v)
	handler := mw(http.HandlerFunc(okHandler))

	body := strings.NewReader(`{"age":42}`) // missing required "name"
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON content-type, got %q", ct)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
}

func TestMiddleware_NoMatchingSchema_PassesThrough(t *testing.T) {
	// Validator with no schemas → ErrNoMatchingSchema → pass through.
	v := apivalidator.NewValidator()
	mw := apivalidator.Middleware(v)
	handler := mw(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 pass-through, got %d", rec.Code)
	}
}

func TestMiddleware_WithOnViolation_Custom(t *testing.T) {
	v := newJSONSchemaValidator(t)

	called := false
	mw := apivalidator.Middleware(v, apivalidator.WithOnViolation(func(w http.ResponseWriter, _ *http.Request, _ *apivalidator.ValidationResult) {
		called = true
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	handler := mw(http.HandlerFunc(okHandler))

	body := strings.NewReader(`{"age":42}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected custom onViolation to be called")
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestMiddleware_WithLogViolations_False(t *testing.T) {
	v := newJSONSchemaValidator(t)
	// Just verify it doesn't panic with logging disabled.
	mw := apivalidator.Middleware(v, apivalidator.WithLogViolations(false))
	handler := mw(http.HandlerFunc(okHandler))

	body := strings.NewReader(`{}`) // invalid
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMiddleware_BodyTooLarge_Returns400(t *testing.T) {
	v, err := apivalidator.NewValidator(apivalidator.WithMaxBodyBytes(5)).
		AddSchemaString(apivalidator.FormatJSONSchema, testSchema)
	if err != nil {
		t.Fatal(err)
	}
	mw := apivalidator.Middleware(v)
	handler := mw(http.HandlerFunc(okHandler))

	body := strings.NewReader(`{"name":"alice and bob"}`) // > 5 bytes
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for body-too-large, got %d", rec.Code)
	}
}

func TestMiddleware_NextNotCalledOnFailure(t *testing.T) {
	v := newJSONSchemaValidator(t)
	mw := apivalidator.Middleware(v)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	body := strings.NewReader(`{}`) // invalid — missing "name"
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Error("next handler should not be called when validation fails")
	}
}

func TestMiddleware_ViolationsInResponse(t *testing.T) {
	v := newJSONSchemaValidator(t)
	mw := apivalidator.Middleware(v)
	handler := mw(http.HandlerFunc(okHandler))

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	b, _ := io.ReadAll(rec.Body)
	var resp map[string]any
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	violations, ok := resp["violations"].([]any)
	if !ok || len(violations) == 0 {
		t.Errorf("expected violations in response, got: %v", resp)
	}
}
