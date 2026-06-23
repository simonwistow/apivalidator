// Package route implements schema-selection logic for the apivalidator.
// It classifies an incoming request into a transport/protocol bucket and
// selects matching schemas from the registered set.
package route

import (
	"net/http"
	"strings"
)

// Transport is the detected wire protocol for a request.
type Transport int

// Transport constants enumerate the detected wire protocol for a request.
const (
	TransportREST    Transport = iota // OpenAPI / RAML / API Blueprint
	TransportGRPC                     // raw gRPC (HTTP/2, Content-Type: application/grpc)
	TransportGRPCWeb                  // gRPC-Web (HTTP/1.1, Content-Type: application/grpc-web)
	TransportGraphQL                  // GraphQL (path heuristic or Content-Type)
	TransportThrift                   // Thrift over HTTP (Content-Type: application/x-thrift)
	TransportUnknown                  // none of the above
)

// DetectTransport inspects Content-Type and path to determine wire protocol.
func DetectTransport(r *http.Request) Transport {
	ct := r.Header.Get("Content-Type")
	// Strip parameters (e.g. "; charset=utf-8").
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}
	ct = strings.ToLower(ct)

	switch {
	case ct == "application/grpc":
		return TransportGRPC
	case ct == "application/grpc-web" || ct == "application/grpc-web+proto":
		return TransportGRPCWeb
	case ct == "application/x-thrift":
		return TransportThrift
	case isGraphQL(r):
		return TransportGraphQL
	default:
		return TransportREST
	}
}

// isGraphQL returns true when the request looks like a GraphQL request.
// Matches: path segment "/graphql", Content-Type "application/graphql", or
// "application/json" POST to a path ending in "/graphql".
func isGraphQL(r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if ct == "application/graphql" {
		return true
	}
	path := strings.ToLower(r.URL.Path)
	return strings.HasSuffix(path, "/graphql") || path == "/graphql"
}

// IsGRPCPath returns true when the path matches the gRPC canonical form
// /package.Service/Method — used as a secondary signal when Content-Type
// is absent (e.g. gRPC-Gateway transcoding).
func IsGRPCPath(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 {
		return false
	}
	// First segment must contain a dot (package.Service).
	return strings.Contains(parts[0], ".")
}
