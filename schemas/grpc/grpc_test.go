package grpc_test

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/schemas/grpc"
	apivalidator "github.com/simonwistow/apivalidator"
)

const helloProto = `
syntax = "proto3";
package hello;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatGRPC, []byte(helloProto)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

// grpcFrame wraps msg bytes in a 5-byte gRPC frame header.
func grpcFrame(msg []byte) []byte {
	frame := make([]byte, 5+len(msg))
	frame[0] = 0x00 // not compressed
	binary.BigEndian.PutUint32(frame[1:], uint32(len(msg)))
	copy(frame[5:], msg)
	return frame
}

func TestGRPC_KnownMethod(t *testing.T) {
	v := newValidator(t)
	// Empty HelloRequest is valid proto3 (all fields optional).
	body := grpcFrame(nil)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/grpc")

	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got: %s", res.Error())
	}
}

func TestGRPC_UnknownMethod(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/UnknownRPC", nil)
	req.Header.Set("Content-Type", "application/grpc")
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for unknown method, got %v", err)
	}
}

func TestGRPC_UnknownService(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/other.Service/SayHello", nil)
	req.Header.Set("Content-Type", "application/grpc")
	_, err := v.ValidateRequest(req)
	if err != apivalidator.ErrNoMatchingSchema {
		t.Errorf("expected ErrNoMatchingSchema for unknown service, got %v", err)
	}
}

func TestGRPC_InvalidFrame(t *testing.T) {
	v := newValidator(t)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello",
		bytes.NewReader([]byte{0x01, 0x00, 0x00})) // too short
	req.Header.Set("Content-Type", "application/grpc")

	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for malformed gRPC frame")
	}
}

func TestGRPCWeb_DetectedTransport(t *testing.T) {
	v := newValidator(t)
	body := grpcFrame(nil)
	req := httptest.NewRequest(http.MethodPost, "/hello.Greeter/SayHello", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/grpc-web+proto")

	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid gRPC-Web request, got: %s", res.Error())
	}
}
