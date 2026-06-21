// Log replay example: validate response bodies captured from production traffic.
//
// When only response bodies are available (e.g. from an API gateway log),
// ValidateResponseBody lets you validate without the original request context.
// This is useful for offline contract testing of captured traffic.
//
// Run with:
//
//	go run ./examples/log_replay
package main

import (
	"fmt"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
)

const responseSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "UserResponse",
	"type": "object",
	"required": ["id", "name"],
	"properties": {
		"id":    {"type": "integer"},
		"name":  {"type": "string"},
		"email": {"type": "string"}
	}
}`

// capturedResponse represents a single entry from an API log.
type capturedResponse struct {
	label       string
	contentType string
	body        []byte
}

func main() {
	// Scope the JSON Schema to responses only — no request validation needed here.
	v, err := apivalidator.NewValidator(
		apivalidator.WithJSONSchemaScope(apivalidator.JSONSchemaResponse),
	).AddSchemaString(apivalidator.FormatJSONSchema, responseSchema)
	if err != nil {
		panic(err)
	}

	// Simulate captured response bodies from an API log.
	entries := []capturedResponse{
		{
			label:       "valid user object",
			contentType: "application/json",
			body:        []byte(`{"id":1,"name":"Alice","email":"alice@example.com"}`),
		},
		{
			label:       "missing required id field",
			contentType: "application/json",
			body:        []byte(`{"name":"Bob"}`),
		},
		{
			label:       "wrong type for id",
			contentType: "application/json",
			body:        []byte(`{"id":"not-a-number","name":"Carol"}`),
		},
		{
			label:       "non-JSON response (ignored)",
			contentType: "text/plain",
			body:        []byte(`OK`),
		},
	}

	violations := 0
	for _, entry := range entries {
		result, err := v.ValidateResponseBody(entry.body, entry.contentType)
		if err == apivalidator.ErrNoMatchingSchema {
			fmt.Printf("[%s] skipped (no matching schema)\n", entry.label)
			continue
		}
		if err != nil {
			fmt.Printf("[%s] error: %v\n", entry.label, err)
			continue
		}
		if result.Valid {
			fmt.Printf("[%s] OK\n", entry.label)
		} else {
			fmt.Printf("[%s] FAIL: %s\n", entry.label, result.Error())
			violations += len(result.Violations)
		}
	}

	fmt.Printf("\ntotal violations: %d\n", violations)
}
