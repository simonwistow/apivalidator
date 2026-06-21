package graphql_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
)

func TestGraphQL_Name_Fingerprint_Format(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatGraphQL, bookSchema)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name")
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
	if schema.Format() != apivalidator.FormatGraphQL {
		t.Errorf("expected FormatGraphQL, got %v", schema.Format())
	}
}

func TestGraphQL_ParseInvalid(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatGraphQL, `type Broken {`)
	if err == nil {
		t.Error("expected error for invalid GraphQL SDL")
	}
}

func TestGraphQL_ValidateRequest_ValidateBodyFalse(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatGraphQL, []byte(bookSchema)); err != nil {
		t.Fatal(err)
	}
	// Body is garbage but validation is disabled.
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte("garbage")))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with body validation off, got: %s", res.Error())
	}
}

func TestGraphQL_ValidateRequest_GETQueryParam(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query=%7B+books+%7B+id+%7D+%7D", nil)
	req.Header.Set("Content-Type", "application/graphql")
	// Set a non-empty body so the body check doesn't bail out
	req.Body = io.NopCloser(bytes.NewReader([]byte("placeholder")))
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for GET query param request, got: %s", res.Error())
	}
}

func TestGraphQL_ValidateRequest_InvalidJSON(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/graphql",
		bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for non-JSON body")
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeInvalidBody {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeInvalidBody violation")
	}
}

func TestGraphQL_ValidateResponse_EmptyBody(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id } }`)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(nil)),
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

func TestGraphQL_ValidateResponse_InvalidJSON(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id } }`)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("not json"))),
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

func TestGraphQL_ValidateResponse_NoDataOrErrors(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id } }`)
	body, _ := json.Marshal(map[string]any{"meta": "only"})
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for response with neither data nor errors")
	}
}

func TestGraphQL_ValidateResponse_WithErrors(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id } }`)
	body, _ := json.Marshal(map[string]any{
		"data":   nil,
		"errors": []any{map[string]any{"message": "book not found"}},
	})
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Errors in response are warnings — still valid.
	if !res.Valid {
		t.Errorf("expected valid (errors are warnings), got: %s", res.Error())
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeInvalidResponse {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeInvalidResponse violation for GraphQL errors")
	}
}

func TestGraphQL_ValidateResponse_ValidateBodyFalse(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatGraphQL, []byte(bookSchema)); err != nil {
		t.Fatal(err)
	}
	req := gqlRequest(`{ books { id } }`)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("garbage"))),
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

func TestGraphQL_ValidateResponse_NilBody(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id } }`)
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
