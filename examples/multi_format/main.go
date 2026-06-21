// Multi-format example: register schemas of several formats in one validator.
//
// Import the "all" package to register every format at once, then add
// schemas as needed. The validator automatically routes each request to the
// correct schema based on Content-Type and path.
//
// Run with:
//
//	go run ./examples/multi_format
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/all" // registers every format
)

const openAPISchema = `{
	"openapi": "3.0.0",
	"info": {"title": "REST API", "version": "1.0.0"},
	"paths": {
		"/items": {
			"get": {
				"responses": {"200": {"description": "ok"}}
			}
		}
	}
}`

const grpcProto = `
syntax = "proto3";
package items;

service ItemService {
  rpc GetItem (GetItemRequest) returns (Item);
}

message GetItemRequest { string id = 1; }
message Item { string id = 1; string name = 2; }
`

const graphqlSDL = `
type Query {
  item(id: ID!): Item
}
type Item {
  id: ID!
  name: String!
}
`

func main() {
	v := apivalidator.NewValidator(
		// Pass through endpoints not covered by any registered schema.
		apivalidator.WithIgnoreUnknownEndpoints(),
	)

	if _, err := v.AddSchemaString(apivalidator.FormatOpenAPI3, openAPISchema); err != nil {
		panic(fmt.Sprintf("openapi: %v", err))
	}
	if _, err := v.AddSchemaString(apivalidator.FormatGRPC, grpcProto); err != nil {
		panic(fmt.Sprintf("grpc: %v", err))
	}
	if _, err := v.AddSchemaString(apivalidator.FormatGraphQL, graphqlSDL); err != nil {
		panic(fmt.Sprintf("graphql: %v", err))
	}

	// REST request → matched by OpenAPI schema.
	rest := httptest.NewRequest(http.MethodGet, "/items", nil)
	rest.Header.Set("Accept", "application/json")
	printResult("REST GET /items", v, rest)

	// gRPC request → matched by gRPC schema.
	idBytes := append([]byte{0x0a, 0x05}, []byte("item1")...)
	frame := grpcFrame(idBytes)
	grpc := httptest.NewRequest(http.MethodPost, "/items.ItemService/GetItem", bytes.NewReader(frame))
	grpc.Header.Set("Content-Type", "application/grpc")
	printResult("gRPC GetItem", v, grpc)

	// GraphQL request → matched by GraphQL schema.
	gql := httptest.NewRequest(http.MethodPost, "/graphql",
		strings.NewReader(`{"query":"{ item(id:\"1\") { id name } }"}`))
	gql.Header.Set("Content-Type", "application/json")
	printResult("GraphQL query", v, gql)

	// Unknown endpoint → ignored due to WithIgnoreUnknownEndpoints.
	unknown := httptest.NewRequest(http.MethodDelete, "/not-registered", nil)
	printResult("unknown endpoint", v, unknown)
}

func printResult(label string, v *apivalidator.Validator, req *http.Request) {
	result, err := v.ValidateRequest(req)
	if err != nil {
		fmt.Printf("[%s] error: %v\n", label, err)
		return
	}
	fmt.Printf("[%s] valid=%v\n", label, result.Valid)
	for _, viol := range result.Violations {
		fmt.Printf("  %s\n", viol)
	}
}

func grpcFrame(msg []byte) []byte {
	frame := make([]byte, 5+len(msg))
	frame[0] = 0x00
	binary.BigEndian.PutUint32(frame[1:], uint32(len(msg)))
	copy(frame[5:], msg)
	return frame
}
