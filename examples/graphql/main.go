// GraphQL example: validate GraphQL POST requests and responses.
//
// GraphQL requests are routed by Content-Type ("application/graphql",
// "application/graphql+json") or by path ending in "/graphql". The body must
// be a JSON object with at least a "query" field; response bodies must contain
// either "data" or "errors".
//
// Run with:
//
//	go run ./examples/graphql
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/graphql"
)

const sdl = `
type Query {
  user(id: ID!): User
  users: [User!]!
}

type User {
  id: ID!
  name: String!
  email: String
}
`

func main() {
	v, err := apivalidator.NewValidator().AddSchemaString(apivalidator.FormatGraphQL, sdl)
	if err != nil {
		panic(err)
	}

	fmt.Println("--- Requests ---")
	for _, tc := range []struct {
		label       string
		contentType string
		body        string
	}{
		{
			"valid query",
			"application/json",
			`{"query":"{ users { id name } }"}`,
		},
		{
			"invalid JSON",
			"application/json",
			`not json at all`,
		},
		{
			"missing query field",
			"application/json",
			`{"operation":"getUser"}`,
		},
	} {
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.contentType)
		result, err := v.ValidateRequest(req)
		if err != nil {
			fmt.Printf("[%s] error: %v\n", tc.label, err)
			continue
		}
		fmt.Printf("[%s] valid=%v\n", tc.label, result.Valid)
		for _, viol := range result.Violations {
			fmt.Printf("  %s\n", viol)
		}
	}

	fmt.Println("\n--- Responses ---")
	for _, tc := range []struct {
		label string
		body  string
	}{
		{"valid data response", `{"data":{"users":[{"id":"1","name":"Alice"}]}}`},
		{"valid error response", `{"errors":[{"message":"not found"}]}`},
		{"invalid: neither data nor errors", `{"status":"ok"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
		req.Header.Set("Content-Type", "application/json")

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(tc.body)),
			Request:    req,
		}
		result, err := v.ValidateResponse(resp, req)
		if err != nil {
			fmt.Printf("[%s] error: %v\n", tc.label, err)
			continue
		}
		fmt.Printf("[%s] valid=%v\n", tc.label, result.Valid)
		for _, viol := range result.Violations {
			fmt.Printf("  %s\n", viol)
		}
	}
}
