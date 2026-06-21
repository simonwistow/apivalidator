// gRPC example: validate a gRPC request against a .proto schema.
//
// gRPC messages are length-prefixed: a 5-byte frame header (1 flag byte +
// 4-byte big-endian length) followed by the protobuf-encoded message body.
// The validator decodes the frame automatically when the Content-Type is
// "application/grpc" or "application/grpc-web[+proto]".
//
// Run with:
//
//	go run ./examples/grpc
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/grpc"
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

func main() {
	v, err := apivalidator.NewValidator().AddSchemaString(apivalidator.FormatGRPC, helloProto)
	if err != nil {
		panic(err)
	}

	// Encode a HelloRequest{name: "world"} in proto3 wire format.
	// Field 1 (string) → tag 0x0a, length, bytes.
	nameBytes := []byte("world")
	protoMsg := append([]byte{0x0a, byte(len(nameBytes))}, nameBytes...)

	for _, tc := range []struct {
		label string
		path  string
		body  []byte
	}{
		{"valid SayHello", "/hello.Greeter/SayHello", grpcFrame(protoMsg)},
		{"unknown method", "/hello.Greeter/Unknown", grpcFrame(protoMsg)},
		{"invalid frame (no header)", "/hello.Greeter/SayHello", protoMsg},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/grpc")
		result, err := v.ValidateRequest(req)
		if err != nil {
			fmt.Printf("[%s] error: %v\n", tc.label, err)
			continue
		}
		fmt.Printf("[%s] valid=%v\n", tc.label, result.Valid)
		for _, v := range result.Violations {
			fmt.Printf("  %s\n", v)
		}
	}
}

// grpcFrame wraps a protobuf message in a 5-byte gRPC frame header.
func grpcFrame(msg []byte) []byte {
	frame := make([]byte, 5+len(msg))
	frame[0] = 0x00 // not compressed
	binary.BigEndian.PutUint32(frame[1:], uint32(len(msg)))
	copy(frame[5:], msg)
	return frame
}
