// Basic example: parse a JSON Schema and validate an HTTP request body.
//
// Run with:
//
//	go run ./examples/basic
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
)

const schema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "CreateUser",
	"type": "object",
	"required": ["name", "email"],
	"properties": {
		"name":  {"type": "string", "minLength": 1},
		"email": {"type": "string", "format": "email"},
		"age":   {"type": "integer", "minimum": 0}
	},
	"additionalProperties": false
}`

func main() {
	// Build a validator with the JSON Schema loaded.
	v, err := apivalidator.NewValidator().AddSchemaString(apivalidator.FormatJSONSchema, schema)
	if err != nil {
		panic(err)
	}

	// --- Valid request ---
	validBody := `{"name":"Alice","email":"alice@example.com","age":30}`
	result, err := validate(v, validBody)
	if err != nil {
		panic(err)
	}
	fmt.Printf("valid body → valid=%v\n", result.Valid)

	// --- Invalid request: missing required field, extra field ---
	invalidBody := `{"email":"bob@example.com","role":"admin"}`
	result, err = validate(v, invalidBody)
	if err != nil {
		panic(err)
	}
	fmt.Printf("invalid body → valid=%v, %d violation(s)\n", result.Valid, len(result.Violations))
	for _, v := range result.Violations {
		fmt.Printf("  %s\n", v)
	}
}

func validate(v *apivalidator.Validator, body string) (*apivalidator.ValidationResult, error) {
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return v.ValidateRequest(req)
}
