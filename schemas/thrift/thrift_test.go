package thrift_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/schemas/thrift"
	apivalidator "github.com/simonwistow/apivalidator"
)

const calculatorThrift = `
namespace go calculator

service Calculator {
  i32 add(1: i32 a, 2: i32 b)
  i32 subtract(1: i32 a, 2: i32 b)
  i32 multiply(1: required i32 a, 2: required i32 b)
}
`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatThrift, []byte(calculatorThrift)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

func TestThrift_IDLParse(t *testing.T) {
	// Parsing the IDL should succeed and find the Calculator service.
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, calculatorThrift)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
}

func TestThrift_EmptyIDL(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, "namespace go empty\n")
	if err == nil {
		t.Error("expected error for IDL with no services")
	}
}

func TestThrift_KnownMethod_NoBody(t *testing.T) {
	v := newValidator(t)
	// No body — validation succeeds structurally.
	req := newRequest("POST", "/Calculator/add", "application/x-thrift", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty body always passes (nothing to validate).
	if !res.Valid {
		t.Errorf("expected valid for empty body, got: %s", res.Error())
	}
}

func TestThrift_UnknownMethod(t *testing.T) {
	v := newValidator(t)
	req := newRequest("POST", "/Calculator/divide", "application/x-thrift", nil)
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for unknown method, got %v", err)
	}
}

func TestThrift_UnknownService(t *testing.T) {
	v := newValidator(t)
	req := newRequest("POST", "/Algebra/add", "application/x-thrift", nil)
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for unknown service, got %v", err)
	}
}

func newRequest(method, path, ct string, body []byte) *http.Request {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", ct)
	return req
}
