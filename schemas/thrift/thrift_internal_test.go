package thrift

import (
	"bytes"
	"context"
	"testing"

	athrift "github.com/apache/thrift/lib/go/thrift"
	apivalidator "github.com/simonwistow/apivalidator"
)

func writeJSONMsg(name string) []byte {
	var buf bytes.Buffer
	transport := athrift.NewStreamTransportW(&buf)
	proto := athrift.NewTJSONProtocol(transport)
	ctx := context.Background()
	_ = proto.WriteMessageBegin(ctx, name, athrift.CALL, 1)
	_ = proto.WriteMessageEnd(ctx)
	// JSON protocol has its own buffer — flush it before flushing the transport.
	_ = proto.Flush(ctx)
	_ = transport.Flush(ctx)
	return buf.Bytes()
}

func writeBinaryMsg(name string) []byte {
	var buf bytes.Buffer
	transport := athrift.NewStreamTransportW(&buf)
	proto := athrift.NewTBinaryProtocol(transport, false, true)
	ctx := context.Background()
	_ = proto.WriteMessageBegin(ctx, name, athrift.CALL, 1)
	_ = proto.WriteMessageEnd(ctx)
	_ = transport.Flush(ctx)
	return buf.Bytes()
}

func newTestSchema(t *testing.T) *thriftSchema {
	t.Helper()
	const idl = `
namespace go test
service Calculator {
  i32 add(1: i32 a, 2: i32 b)
}
`
	parsed, err := parseIDL(idl)
	if err != nil {
		t.Fatalf("parseIDL: %v", err)
	}
	return &thriftSchema{name: "Calculator", idl: parsed}
}

func TestThriftInternal_ValidateThriftJSON_Valid(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	body := writeJSONMsg("add")
	violations := s.validateThriftJSON(body, method)
	if len(violations) != 0 {
		t.Errorf("expected no violations for valid JSON Thrift message, got: %v", violations)
	}
}

func TestThriftInternal_ValidateThriftJSON_WrongMethod(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	body := writeJSONMsg("subtract")
	violations := s.validateThriftJSON(body, method)
	found := false
	for _, v := range violations {
		if v.Code == apivalidator.CodeUnknownOperation {
			found = true
		}
	}
	if !found {
		t.Error("expected CodeUnknownOperation for mismatched JSON method name")
	}
}

func TestThriftInternal_ValidateThriftJSON_Invalid(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	violations := s.validateThriftJSON([]byte(`not json thrift`), method)
	if len(violations) == 0 {
		t.Error("expected violations for invalid JSON Thrift body")
	}
	if violations[0].Code != apivalidator.CodeInvalidBody {
		t.Errorf("expected CodeInvalidBody, got %v", violations[0].Code)
	}
}

func TestThriftInternal_ValidateThriftBinary_Valid(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	body := writeBinaryMsg("add")
	violations := s.validateThriftBinary(body, method)
	if len(violations) != 0 {
		t.Errorf("expected no violations for matching binary Thrift message, got: %v", violations)
	}
}

func TestThriftInternal_ValidateThriftBinary_WrongMethod(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	body := writeBinaryMsg("subtract")
	violations := s.validateThriftBinary(body, method)
	found := false
	for _, v := range violations {
		if v.Code == apivalidator.CodeUnknownOperation {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CodeUnknownOperation for mismatched binary method name, got: %v", violations)
	}
}

func TestThriftInternal_ValidateThriftBinary_Invalid(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	violations := s.validateThriftBinary([]byte{0xff, 0xfe, 0xfd}, method)
	if len(violations) == 0 {
		t.Error("expected violations for invalid binary Thrift body")
	}
}

func TestThriftInternal_ValidateBody_EmptyBody(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	violations := s.validateThriftBody(nil, method, "application/x-thrift")
	if len(violations) != 0 {
		t.Errorf("expected nil violations for empty body, got: %v", violations)
	}
}

func TestThriftInternal_ValidateBody_JSONContentType(t *testing.T) {
	s := newTestSchema(t)
	method := s.idl.services["Calculator"].methods["add"]
	body := writeJSONMsg("add")
	violations := s.validateThriftBody(body, method, "application/json")
	if len(violations) != 0 {
		t.Errorf("expected no violations for valid JSON body with JSON CT, got: %v", violations)
	}
}

func TestThriftInternal_HasErrors(t *testing.T) {
	cases := []struct {
		vs   []apivalidator.Violation
		want bool
	}{
		{nil, false},
		{[]apivalidator.Violation{{Severity: apivalidator.SeverityWarning}}, false},
		{[]apivalidator.Violation{{Severity: apivalidator.SeverityError}}, true},
		{[]apivalidator.Violation{
			{Severity: apivalidator.SeverityWarning},
			{Severity: apivalidator.SeverityError},
		}, true},
	}
	for _, tc := range cases {
		if got := hasErrors(tc.vs); got != tc.want {
			t.Errorf("hasErrors(%v) = %v, want %v", tc.vs, got, tc.want)
		}
	}
}
