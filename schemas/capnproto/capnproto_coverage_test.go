package capnproto_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
)

func TestCapnProto_Name_Fingerprint_Format(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatCapnProto, addressBookSchema)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name")
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
	if schema.Format() != apivalidator.FormatCapnProto {
		t.Errorf("expected FormatCapnProto, got %v", schema.Format())
	}
}

func TestCapnProto_SchemaWithHint(t *testing.T) {
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaString(apivalidator.FormatCapnProto, addressBookSchema); err != nil {
		t.Fatalf("AddSchemaString: %v", err)
	}
}

func TestCapnProto_ValidateResponse_EmptyBody(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/address", nil)
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
		t.Errorf("expected valid for empty Cap'n Proto response body, got: %s", res.Error())
	}
}

func TestCapnProto_ValidateResponse_ValidBody(t *testing.T) {
	v := newValidator(t)
	body := buildCapnpMessage(t)
	req := httptest.NewRequest(http.MethodPost, "/address", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/octet-stream"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid Cap'n Proto response, got: %s", res.Error())
	}
}

func TestCapnProto_ValidateResponse_InvalidBody(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/address", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0xff, 0xff})),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for malformed Cap'n Proto response body")
	}
}

func TestCapnProto_ValidateResponse_NilBody(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/address", nil)
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

func TestCapnProto_ValidateBody_Disabled(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatCapnProto, []byte(addressBookSchema)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/address", bytes.NewReader([]byte("garbage")))
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid when body validation is off, got: %s", res.Error())
	}
}

func TestCapnProto_SchemaWithComments(t *testing.T) {
	const schemaWithComments = `
@0xdeadbeefcafebabe;

# This is a comment
struct Item {
  # Field comment
  name @0 :Text;
  value @1 :Int32;
}
`
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatCapnProto, schemaWithComments)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
}

func TestCapnProto_SchemaMultipleStructs(t *testing.T) {
	const multiStruct = `
@0xdeadbeefcafebabe;

struct Contact {
  name @0 :Text;
  phone @1 :Text;
}

struct Organization {
  name @0 :Text;
  contacts @1 :List(Contact);
}
`
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatCapnProto, multiStruct)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
}

func TestCapnProto_ParseEmptySchema(t *testing.T) {
	// An empty schema is valid (no structs defined).
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatCapnProto, "@0xdeadbeefcafebabe;\n")
	if err != nil {
		t.Errorf("unexpected error for empty schema: %v", err)
	}
}
