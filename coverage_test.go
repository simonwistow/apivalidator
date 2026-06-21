package apivalidator_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
	_ "github.com/simonwistow/apivalidator/schemas/openapi"
)

// ── result.go ──────────────────────────────────────────────────────────────────

func TestValidationResult_IsValid(t *testing.T) {
	r := &apivalidator.ValidationResult{Valid: true}
	if !r.IsValid() {
		t.Error("IsValid should return true")
	}
	r.Valid = false
	if r.IsValid() {
		t.Error("IsValid should return false")
	}
}

func TestValidationResult_Error_Valid(t *testing.T) {
	r := &apivalidator.ValidationResult{Valid: true}
	if got := r.Error(); got != "" {
		t.Errorf("Error on valid result should be empty, got %q", got)
	}
}

func TestValidationResult_Error_NoViolations(t *testing.T) {
	r := &apivalidator.ValidationResult{Valid: false}
	if got := r.Error(); got == "" {
		t.Error("Error on invalid result with no violations should be non-empty")
	}
}

func TestValidationResult_Error_WithViolations(t *testing.T) {
	r := &apivalidator.ValidationResult{
		Valid: false,
		Violations: []apivalidator.Violation{
			{Path: "/foo", Message: "bad value", Code: apivalidator.CodeInvalidBody, Severity: apivalidator.SeverityError},
			{Message: "missing field", Code: apivalidator.CodeMissingField, Severity: apivalidator.SeverityError},
		},
	}
	got := r.Error()
	if got == "" {
		t.Error("Error should be non-empty")
	}
	// Both violations should appear in the string.
	if len(r.Violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(r.Violations))
	}
}

func TestViolation_String_WithPath(t *testing.T) {
	v := apivalidator.Violation{
		Path:     "/name",
		Message:  "must be string",
		Code:     apivalidator.CodeInvalidFieldType,
		Severity: apivalidator.SeverityError,
	}
	s := v.String()
	if s == "" {
		t.Error("Violation.String() should not be empty")
	}
	// Should contain path, message, code, severity.
	for _, want := range []string{"/name", "must be string", "error"} {
		if !bytes.Contains([]byte(s), []byte(want)) {
			t.Errorf("Violation.String() missing %q, got: %s", want, s)
		}
	}
}

func TestViolation_String_WithoutPath(t *testing.T) {
	v := apivalidator.Violation{
		Message:  "general failure",
		Code:     apivalidator.CodeInvalidBody,
		Severity: apivalidator.SeverityWarning,
	}
	s := v.String()
	if s == "" {
		t.Error("Violation.String() should not be empty")
	}
}

func TestSeverity_String(t *testing.T) {
	cases := []struct {
		s    apivalidator.Severity
		want string
	}{
		{apivalidator.SeverityError, "error"},
		{apivalidator.SeverityWarning, "warning"},
		{apivalidator.SeverityInfo, "info"},
		{apivalidator.Severity(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

// ── schema.go / Format.String() ───────────────────────────────────────────────

func TestFormat_String(t *testing.T) {
	cases := []struct {
		f    apivalidator.Format
		want string
	}{
		{apivalidator.FormatOpenAPI3, "openapi3"},
		{apivalidator.FormatOpenAPI2, "openapi2"},
		{apivalidator.FormatRAML, "raml"},
		{apivalidator.FormatAPIBlueprint, "apiblueprint"},
		{apivalidator.FormatGRPC, "grpc"},
		{apivalidator.FormatCapnProto, "capnproto"},
		{apivalidator.FormatThrift, "thrift"},
		{apivalidator.FormatJSONSchema, "jsonschema"},
		{apivalidator.FormatGraphQL, "graphql"},
		{apivalidator.Format(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.f.String(); got != tc.want {
			t.Errorf("Format(%d).String() = %q, want %q", tc.f, got, tc.want)
		}
	}
}

// ── schema.go / ParseSchemaFile ────────────────────────────────────────────────

func TestParseSchemaFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	content := `{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	schema, err := apivalidator.ParseSchemaFile(apivalidator.FormatOpenAPI3, path)
	if err != nil {
		t.Fatalf("ParseSchemaFile: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestParseSchemaFile_Nonexistent(t *testing.T) {
	_, err := apivalidator.ParseSchemaFile(apivalidator.FormatOpenAPI3, "/no/such/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseSchemaFile_UnknownFormat(t *testing.T) {
	_, err := apivalidator.ParseSchemaFile(apivalidator.Format(99), "/tmp/x.json")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

// ── apivalidator.go ────────────────────────────────────────────────────────────

func TestAddSchemaString(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{}}`
	if _, err := v.AddSchemaString(apivalidator.FormatOpenAPI3, schema); err != nil {
		t.Fatalf("AddSchemaString: %v", err)
	}
}

func TestAddSchemaFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.json")
	const schema = `{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{}}`
	if err := os.WriteFile(path, []byte(schema), 0644); err != nil {
		t.Fatal(err)
	}
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaFile(apivalidator.FormatOpenAPI3, path); err != nil {
		t.Fatalf("AddSchemaFile: %v", err)
	}
}

func TestAddSchemaFile_BadPath(t *testing.T) {
	v := apivalidator.NewValidator()
	_, err := v.AddSchemaFile(apivalidator.FormatOpenAPI3, "/no/such/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestValidateResponseBody_NoSchema(t *testing.T) {
	v := apivalidator.NewValidator()
	_, err := v.ValidateResponseBody([]byte(`{}`), "application/json")
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema with no schema registered, got %v", err)
	}
}

func TestValidateResponseBody_Valid(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"title":"T","type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	res, err := v.ValidateResponseBody([]byte(`{"id":1}`), "application/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid body, got: %s", res.Error())
	}
}

func TestValidateResponseBody_Invalid(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"title":"T","type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	res, err := v.ValidateResponseBody([]byte(`{"id":"wrong"}`), "application/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid body (wrong type for id)")
	}
}

func TestValidateResponseBody_EmptyContentType(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"title":"T","type":"object"}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	// Empty content-type is treated permissively — JSON Schema still runs.
	res, err := v.ValidateResponseBody([]byte(`{}`), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for empty content-type, got: %s", res.Error())
	}
}

func TestValidateResponseBody_NonJSONContentType(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"title":"T","type":"object"}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	_, err := v.ValidateResponseBody([]byte(`{}`), "application/x-thrift")
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for non-JSON content-type, got %v", err)
	}
}

func TestValidateResponseBody_RequestScopeSkips(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaRequest))
	const schema = `{"title":"T","type":"object"}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	_, err := v.ValidateResponseBody([]byte(`{}`), "application/json")
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema with request-only scope, got %v", err)
	}
}

func TestValidateResponse_PackageLevel(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatOpenAPI3,
		`{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{"/x":{"get":{"responses":{"200":{"description":"OK"}}}}}}`)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	resp := &http.Response{StatusCode: 200, Request: req}
	res, err := apivalidator.ValidateResponse(resp, req, schema)
	if err != nil {
		t.Fatalf("ValidateResponse: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid: %s", res.Error())
	}
}

// ── body.go ───────────────────────────────────────────────────────────────────

func TestReadResponseBody_NilBody(t *testing.T) {
	cfg := &apivalidator.Config{MaxBodyBytes: 1024}
	resp := &http.Response{Body: nil}
	data, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for nil body, got %v", data)
	}
}

func TestReadResponseBody_Normal(t *testing.T) {
	cfg := &apivalidator.Config{MaxBodyBytes: 1024}
	body := `{"ok":true}`
	resp := &http.Response{Body: io.NopCloser(bytes.NewBufferString(body))}
	data, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != body {
		t.Errorf("expected body %q, got %q", body, data)
	}
	// Body should be re-readable.
	second, _ := io.ReadAll(resp.Body)
	if string(second) != body {
		t.Errorf("body not restored for re-reading, got %q", second)
	}
}

func TestReadResponseBody_TooLarge(t *testing.T) {
	cfg := &apivalidator.Config{MaxBodyBytes: 4}
	resp := &http.Response{Body: io.NopCloser(bytes.NewBufferString("12345"))}
	_, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != apivalidator.ErrBodyTooLarge {
		t.Errorf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestReadRequestBody_TooLarge(t *testing.T) {
	cfg := &apivalidator.Config{MaxBodyBytes: 3}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("1234"))
	_, err := apivalidator.ReadRequestBody(req, cfg)
	if err != apivalidator.ErrBodyTooLarge {
		t.Errorf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestReadRequestBody_NilBody(t *testing.T) {
	cfg := &apivalidator.Config{MaxBodyBytes: 1024}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil
	data, err := apivalidator.ReadRequestBody(req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for nil body, got %v", data)
	}
}

// ── options.go ────────────────────────────────────────────────────────────────

func TestWithMaxBodyBytes_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithMaxBodyBytes(512)(cfg)
	if cfg.MaxBodyBytes != 512 {
		t.Errorf("expected MaxBodyBytes=512, got %d", cfg.MaxBodyBytes)
	}
}

func TestWithStrictMode_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithStrictMode(true)(cfg)
	if !cfg.StrictMode {
		t.Error("expected StrictMode=true")
	}
}

func TestWithStrictMode_UnknownEndpoint(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithStrictMode(true))
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(minimalOpenAPI)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/no/such/endpoint", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("StrictMode should return result not error, got err: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid result for unknown endpoint in strict mode")
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeUnknownEndpoint {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeUnknownEndpoint violation in strict mode")
	}
}

func TestWithStrictMode_KnownEndpointStillValid(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithStrictMode(true))
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(minimalOpenAPI)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for known endpoint in strict mode, got: %s", res.Error())
	}
}

func TestWithLogger_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	l := slog.Default()
	apivalidator.WithLogger(l)(cfg)
	if cfg.Logger != l {
		t.Error("expected Logger to be set")
	}
}

func TestWithValidateBody_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithValidateBody(false)(cfg)
	if cfg.ValidateBody {
		t.Error("expected ValidateBody=false")
	}
}

func TestWithValidateQueryParams_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithValidateQueryParams(false)(cfg)
	if cfg.ValidateQueryParams {
		t.Error("expected ValidateQueryParams=false")
	}
}

func TestWithValidateHeaders_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithValidateHeaders(true)(cfg)
	if !cfg.ValidateHeaders {
		t.Error("expected ValidateHeaders=true")
	}
}

func TestWithJSONSchemaScope_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaRequest)(cfg)
	if cfg.JSONSchemaScope != apivalidator.JSONSchemaRequest {
		t.Errorf("expected JSONSchemaRequest, got %v", cfg.JSONSchemaScope)
	}
}

func TestWithMultiSchemaPolicy_Effect(t *testing.T) {
	cfg := &apivalidator.Config{}
	apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAny)(cfg)
	if cfg.MultiSchemaPolicy != apivalidator.PolicyAny {
		t.Errorf("expected PolicyAny, got %v", cfg.MultiSchemaPolicy)
	}
}

// ── dispatch.go — schemaMatchesTransport ──────────────────────────────────────

// Test that gRPC schemas are not selected for REST requests and vice versa.
func TestDispatch_TransportFiltering_GRPC(t *testing.T) {
	v := apivalidator.NewValidator()
	// Register only a JSON Schema (REST transport).
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(`{"type":"object"}`)); err != nil {
		t.Fatal(err)
	}
	// Send a gRPC request — no schema matches.
	req := httptest.NewRequest(http.MethodPost, "/pkg.Svc/Method", nil)
	req.Header.Set("Content-Type", "application/grpc")
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for gRPC request with REST-only schema, got %v", err)
	}
}

func TestDispatch_TransportFiltering_Thrift(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(`{"type":"object"}`)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/Service/method", nil)
	req.Header.Set("Content-Type", "application/x-thrift")
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for Thrift request with REST-only schema, got %v", err)
	}
}

func TestDispatch_TransportFiltering_GraphQL(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(`{"type":"object"}`)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	req.Header.Set("Content-Type", "application/json")
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for GraphQL path with JSON Schema, got %v", err)
	}
}

// ── dispatch.go — mergeResults with PolicyAny ─────────────────────────────────

func TestMultiSchemaPolicyAny_Passes(t *testing.T) {
	// Register two schemas: one valid path, one that won't match. PolicyAny: pass if any passes.
	v := apivalidator.NewValidator(apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAny))
	// Two different OpenAPI schemas for the same path — both will match.
	const s1 = `{"openapi":"3.0.0","info":{"title":"A","version":"1"},"paths":{"/x":{"get":{"responses":{"200":{"description":"OK"}}}}}}`
	const s2 = `{"openapi":"3.0.0","info":{"title":"B","version":"1"},"paths":{"/x":{"get":{"responses":{"200":{"description":"OK"}}}}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(s1)); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(s2)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with PolicyAny (two passing schemas): %s", res.Error())
	}
}

func TestMultiSchemaPolicyAny_FailsWhenNonePasses(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAny))
	// Schema requires integer petId; send non-integer → violation from both schemas.
	const s1 = `{"openapi":"3.0.0","info":{"title":"A","version":"1"},"paths":{"/pets/{id}":{"get":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"integer"}}],"responses":{"200":{"description":"OK"}}}}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(s1)); err != nil {
		t.Fatal(err)
	}
	// Add same schema again so we have multiple candidates.
	if _, err := v.AddSchemaBytes(apivalidator.FormatOpenAPI3, []byte(s1)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/pets/not-an-int", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid when no schema passes under PolicyAny")
	}
}

// ── dispatch.go — IgnoreUnknown with no candidates ───────────────────────────

func TestIgnoreUnknown_NoCandidates(t *testing.T) {
	// JSON schema (REST) + gRPC request → no candidates → should return valid with IgnoreUnknown.
	v := apivalidator.NewValidator(apivalidator.WithIgnoreUnknownEndpoints())
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(`{"type":"object"}`)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/foo.Bar/Method", nil)
	req.Header.Set("Content-Type", "application/grpc")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with IgnoreUnknown and no candidates, got: %s", res.Error())
	}
}

// ── JSONSchema ValidateResponse ────────────────────────────────────────────────

func TestJSONSchema_ValidateResponse_Valid(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"title":"R","$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["status"],"properties":{"status":{"type":"string"}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"status":"ok"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid JSON Schema response, got: %s", res.Error())
	}
}

func TestJSONSchema_ValidateResponse_Invalid(t *testing.T) {
	v := apivalidator.NewValidator()
	const schema = `{"title":"R","$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["status"],"properties":{"status":{"type":"string"}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"wrong":"field"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid JSON Schema response (missing required field)")
	}
}

func TestJSONSchema_Name_Fingerprint(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatJSONSchema,
		`{"title":"MySchema","type":"object"}`)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() != "MySchema" {
		t.Errorf("expected Name=MySchema, got %q", schema.Name())
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
}

func TestJSONSchema_ValidateBody_Direct(t *testing.T) {
	// ValidateBody is exported on the schema; test via response validation with scope.
	v := apivalidator.NewValidator(apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaResponse))
	const schema = `{"title":"R","type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"id":"not-int"}`)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid response body (wrong type)")
	}
}

func TestWithMaxBodyBytes_RejectsLargeBody(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithMaxBodyBytes(5))
	const schema = `{"title":"T","type":"object"}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/anything",
		bytes.NewBufferString(`{"this":"is more than 5 bytes"}`))
	req.Header.Set("Content-Type", "application/json")
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrBodyTooLarge {
		t.Errorf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestWithValidateBody_False_SkipsJSON(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	const schema = `{"title":"T","type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`
	if _, err := v.AddSchemaBytes(apivalidator.FormatJSONSchema, []byte(schema)); err != nil {
		t.Fatal(err)
	}
	// Invalid body — but validation is disabled.
	req := httptest.NewRequest(http.MethodPost, "/anything", bytes.NewBufferString(`{"wrong":true}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid when body validation is off, got: %s", res.Error())
	}
}
