// Migration example: validate against two API versions simultaneously using PolicyAny.
//
// During a schema migration you can register both the old and new schema.
// PolicyAny passes the request if it satisfies at least one of them — so
// clients can migrate independently without breaking validation.
//
// Run with:
//
//	go run ./examples/migration
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
)

// v1: users are identified by a numeric id field.
const schemaV1 = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "Users API v1",
	"type": "object",
	"required": ["id", "name"],
	"properties": {
		"id":   {"type": "integer"},
		"name": {"type": "string"}
	},
	"additionalProperties": false
}`

// v2: users are identified by a string uuid field instead.
const schemaV2 = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "Users API v2",
	"type": "object",
	"required": ["uuid", "name"],
	"properties": {
		"uuid": {"type": "string"},
		"name": {"type": "string"}
	},
	"additionalProperties": false
}`

func main() {
	v := apivalidator.NewValidator(
		apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAny),
	)
	if _, err := v.AddSchemaString(apivalidator.FormatJSONSchema, schemaV1); err != nil {
		panic(fmt.Sprintf("load v1: %v", err))
	}
	if _, err := v.AddSchemaString(apivalidator.FormatJSONSchema, schemaV2); err != nil {
		panic(fmt.Sprintf("load v2: %v", err))
	}

	for _, tc := range []struct {
		label string
		body  string
	}{
		{"v1 client (id+name)", `{"id":1,"name":"Alice"}`},
		{"v2 client (uuid+name)", `{"uuid":"abc-123","name":"Bob"}`},
		{"neither version", `{"username":"charlie"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
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
}
