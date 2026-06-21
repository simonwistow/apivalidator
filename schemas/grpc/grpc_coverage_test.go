package grpc_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apivalidator "github.com/simonwistow/apivalidator"
)

// grpcFrameMsg wraps proto bytes in a 5-byte gRPC frame.
func grpcFrameMsg(msg []byte) []byte {
	frame := make([]byte, 5+len(msg))
	frame[0] = 0x00
	binary.BigEndian.PutUint32(frame[1:], uint32(len(msg)))
	copy(frame[5:], msg)
	return frame
}

// Encode a simple proto3 string field (field 1, wire type 2).
func protoStringField1(val string) []byte {
	// Tag: field 1, wire type 2 → 0x0a
	data := []byte{0x0a, byte(len(val))}
	return append(data, []byte(val)...)
}

func TestGRPC_Name_Fingerprint(t *testing.T) {
	schema, err := apivalidator.ParseSchemaString(apivalidator.FormatGRPC, helloProto)
	if err != nil {
		t.Fatalf("ParseSchemaString: %v", err)
	}
	if schema.Name() == "" {
		t.Error("expected non-empty Name")
	}
	if schema.Fingerprint() == "" {
		t.Error("expected non-empty Fingerprint")
	}
	if schema.Format() != apivalidator.FormatGRPC {
		t.Errorf("expected FormatGRPC, got %v", schema.Format())
	}
}

func TestGRPC_ParseInvalid(t *testing.T) {
	_, err := apivalidator.ParseSchemaString(apivalidator.FormatGRPC, `not proto syntax %%%`)
	if err == nil {
		t.Error("expected error for invalid proto syntax")
	}
}

func TestGRPC_ValidateRequest_WithProtoBody(t *testing.T) {
	v := newValidator(t)
	// HelloRequest with name = "world"
	protoBytes := protoStringField1("world")
	body := grpcFrameMsg(protoBytes)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/grpc")

	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for valid proto body, got: %s", res.Error())
	}
}

func TestGRPC_ValidateRequest_ValidateBodyFalse(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatGRPC, []byte(helloProto)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", bytes.NewReader([]byte("junk")))
	req.Header.Set("Content-Type", "application/grpc")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid when body validation is off, got: %s", res.Error())
	}
}

func TestGRPC_ValidateResponse_EmptyBody(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", nil)
	req.Header.Set("Content-Type", "application/grpc")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/grpc"}},
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

func TestGRPC_ValidateResponse_ValidFrame(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", nil)
	req.Header.Set("Content-Type", "application/grpc")
	// HelloReply with message = "hi"
	protoBytes := protoStringField1("hi")
	body := grpcFrameMsg(protoBytes)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/grpc"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid for valid gRPC response body, got: %s", res.Error())
	}
}

func TestGRPC_ValidateResponse_InvalidFrame(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", nil)
	req.Header.Set("Content-Type", "application/grpc")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/grpc"}},
		Body:       io.NopCloser(bytes.NewReader([]byte{0x01, 0x00})), // too short
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for malformed gRPC response frame")
	}
}

func TestGRPC_ValidateResponse_UnknownMethod(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/NoSuchRPC", nil)
	req.Header.Set("Content-Type", "application/grpc")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/grpc"}},
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Request:    req,
	}
	_, err := v.ValidateResponse(resp, req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for unknown method in response, got %v", err)
	}
}

func TestGRPC_ValidateResponse_ValidateBodyFalse(t *testing.T) {
	v := apivalidator.NewValidator(apivalidator.WithValidateBody(false))
	if _, err := v.AddSchemaBytes(apivalidator.FormatGRPC, []byte(helloProto)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", nil)
	req.Header.Set("Content-Type", "application/grpc")
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("junk"))),
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

func TestGRPCWeb_TrailerFrame(t *testing.T) {
	v := newValidator(t)
	// gRPC-Web trailer frame: flag byte 0x80
	trailerFrame := []byte{0x80, 0x00, 0x00, 0x00, 0x00}
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello",
		bytes.NewReader(trailerFrame))
	req.Header.Set("Content-Type", "application/grpc-web+proto")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Trailer frames return nil body → valid.
	if !res.Valid {
		t.Errorf("expected valid for gRPC-Web trailer frame, got: %s", res.Error())
	}
}

func TestGRPC_CompressedFrame_Invalid(t *testing.T) {
	v := newValidator(t)
	// Compressed flag (0x01) — not supported
	compressedFrame := []byte{0x01, 0x00, 0x00, 0x00, 0x00}
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello",
		bytes.NewReader(compressedFrame))
	req.Header.Set("Content-Type", "application/grpc")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for compressed gRPC frame")
	}
}

func TestGRPC_FrameLengthExceedsData(t *testing.T) {
	v := newValidator(t)
	// Frame says 100 bytes but only 0 follow
	frame := []byte{0x00, 0x00, 0x00, 0x00, 0x64} // len=100, no data
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello",
		bytes.NewReader(frame))
	req.Header.Set("Content-Type", "application/grpc")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for frame with length exceeding data")
	}
}
