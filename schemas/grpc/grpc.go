// Package grpc implements validation for gRPC services described by .proto files.
// Transport auto-detection covers both raw gRPC (HTTP/2, Content-Type: application/grpc)
// and gRPC-Web (HTTP/1.1, Content-Type: application/grpc-web[+proto]).
// Implemented in Phase 4.
package grpc
