// Middleware example: validate every incoming request automatically.
//
// Run with:
//
//	go run ./examples/middleware
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema"
)

const schema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"required": ["message"],
	"properties": {
		"message": {"type": "string"}
	}
}`

func main() {
	v, err := apivalidator.NewValidator().AddSchemaString(apivalidator.FormatJSONSchema, schema)
	if err != nil {
		panic(err)
	}

	// Wrap your application handler with the validation middleware.
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"status":"ok"}`)
	})
	handler := apivalidator.Middleware(v)(appHandler)

	// Simulate requests against the wrapped handler.
	for _, tc := range []struct {
		label string
		body  string
	}{
		{"valid", `{"message":"hello world"}`},
		{"missing required field", `{"count":42}`},
		{"empty body", ``},
	} {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		resp := rec.Result()
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("[%s] status=%d body=%s\n", tc.label, resp.StatusCode, strings.TrimSpace(string(b)))
	}
}
