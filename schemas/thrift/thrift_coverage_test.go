package thrift_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	athrift "github.com/apache/thrift/lib/go/thrift"
	apivalidator "github.com/simonwistow/apivalidator"
)

// writeBinaryThriftMessage encodes a Thrift binary CALL message.
func writeBinaryThriftMessage(name string) []byte {
	var buf bytes.Buffer
	transport := athrift.NewStreamTransportW(&buf)
	proto := athrift.NewTBinaryProtocol(transport, false, true)
	ctx := context.Background()
	_ = proto.WriteMessageBegin(ctx, name, athrift.CALL, 1)
	_ = proto.WriteMessageEnd(ctx)
	_ = transport.Flush(ctx)
	return buf.Bytes()
}

// writeJSONThriftMessage encodes a Thrift JSON CALL message.
func writeJSONThriftMessage(name string) []byte {
	var buf bytes.Buffer
	transport := athrift.NewStreamTransportW(&buf)
	proto := athrift.NewTJSONProtocol(transport)
	ctx := context.Background()
	_ = proto.WriteMessageBegin(ctx, name, athrift.CALL, 1)
	_ = proto.WriteMessageEnd(ctx)
	_ = transport.Flush(ctx)
	return buf.Bytes()
}

func TestThrift_Name_Fingerprint(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, calculatorThrift)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name")
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
	if schema.Format() != apivalidator.FormatThrift {
		t.Errorf("expected FormatThrift, got %v", schema.Format())
	}
}

func TestThrift_ValidateResponse(t *testing.T) {
	v := newValidator(t)
	req := newRequest("POST", "/Calculator/add", "application/x-thrift", nil)
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
		t.Errorf("expected valid Thrift response, got: %s", res.Error())
	}
}

func TestThrift_ValidateResponse_UnknownPath(t *testing.T) {
	v := newValidator(t)
	req := newRequest("POST", "/", "application/x-thrift", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Request:    req,
	}
	_, err := v.ValidateResponse(resp, req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for empty path in response, got %v", err)
	}
}

func TestThrift_Binary_ValidMessage(t *testing.T) {
	v := newValidator(t)
	body := writeBinaryThriftMessage("add")
	req := newRequest("POST", "/Calculator/add", "application/x-thrift", body)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for valid binary Thrift message, got: %s", res.Error())
	}
}

func TestThrift_Binary_WrongMethodName(t *testing.T) {
	v := newValidator(t)
	// Message body says "multiply" but path says "add"
	body := writeBinaryThriftMessage("multiply")
	req := newRequest("POST", "/Calculator/add", "application/x-thrift", body)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mismatched name is a warning — still valid.
	if !res.Valid {
		t.Errorf("expected valid (mismatched name is warning), got: %s", res.Error())
	}
	found := false
	for _, viol := range res.Violations {
		if viol.Code == apivalidator.CodeUnknownOperation {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeUnknownOperation violation for mismatched method name")
	}
}

func TestThrift_Binary_InvalidBody(t *testing.T) {
	v := newValidator(t)
	body := []byte{0xff, 0xfe, 0xfd} // garbage
	req := newRequest("POST", "/Calculator/add", "application/x-thrift", body)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for garbage binary Thrift body")
	}
}


func TestThrift_ValidateBody_Disabled(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatThrift, []byte(calculatorThrift)); err != nil {
		t.Fatal(err)
	}
	body := []byte("garbage")
	req := newRequest("POST", "/Calculator/add", "application/x-thrift", body)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid with body validation off, got: %s", res.Error())
	}
}

func TestThrift_PathForms(t *testing.T) {
	v := newValidator(t)
	// service.method form
	req := newRequest("POST", "/Calculator.add", "application/x-thrift", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for service.method path form, got: %s", res.Error())
	}
}

func TestThrift_IDL_WithStructsAndEnums(t *testing.T) {
	const complexThrift = `
# This is a comment
// Another comment
/* Block comment */

namespace go myapp

const i32 VERSION = 1

enum Status {
  ACTIVE = 1,
  INACTIVE = 2
}

struct Address {
  1: required string street,
  2: optional string city = "NYC",
  3: i32 zip
}

exception ServiceError {
  1: string message,
  2: i32 code
}

service MyService {
  string getStatus(1: required string id)
  void update(1: Address addr) throws (1: ServiceError err)
}
`
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, complexThrift)
	if err != nil {
		t.Fatalf("ParseSchemaString for complex IDL: %v", err)
	}
}

func TestThrift_IDL_ExtendedService(t *testing.T) {
	const extendedThrift = `
namespace go ext

service BaseService {
  string ping()
}

service ExtendedService extends BaseService {
  string hello(1: string name)
}
`
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, extendedThrift)
	if err != nil {
		t.Fatalf("ParseSchemaString for extended service: %v", err)
	}
}

func TestThrift_IDL_OnewayMethod(t *testing.T) {
	const onewayThrift = `
namespace go oneway

service Queue {
  oneway void push(1: string item)
  string pop()
}
`
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, onewayThrift)
	if err != nil {
		t.Fatalf("ParseSchemaString for oneway method: %v", err)
	}
}

func TestThrift_IDL_GenericFieldTypes(t *testing.T) {
	// Generic types in field positions (not return positions) are supported.
	const genericThrift = `
namespace go generic

struct Response {
  1: list<string> items,
  2: map<string, i32> counts,
  3: set<string> tags
}

service DataService {
  string getItems()
  string getCounts(1: string key)
}
`
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatThrift, genericThrift)
	if err != nil {
		t.Fatalf("ParseSchemaString for generic field types: %v", err)
	}
}

func TestThrift_SingleSegmentPath(t *testing.T) {
	// "method-only" path form: /add
	const singleSvcThrift = `
namespace go single
service Simple {
  i32 add(1: i32 a, 2: i32 b)
}
`
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatThrift, []byte(singleSvcThrift)); err != nil {
		t.Fatal(err)
	}
	// Method-only form — service name is empty, searches all services.
	req := newRequest("POST", "/add", "application/x-thrift", nil)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for method-only path, got: %s", res.Error())
	}
}
