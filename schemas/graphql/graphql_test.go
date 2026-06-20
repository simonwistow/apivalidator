package graphql_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/simonwistow/apivalidator/schemas/graphql"
	apivalidator "github.com/simonwistow/apivalidator"
)

const bookSchema = `
type Query {
  book(id: ID!): Book
  books: [Book]
}
type Book {
  id: ID!
  title: String!
  author: String
}
`

func newValidator(t *testing.T) *apivalidator.Validator {
	t.Helper()
	v := apivalidator.NewValidator()
	if _, err := v.AddSchemaBytes(apivalidator.FormatGraphQL, []byte(bookSchema)); err != nil {
		t.Fatalf("AddSchemaBytes: %v", err)
	}
	return v
}

func gqlRequest(query string) *http.Request {
	body, _ := json.Marshal(map[string]string{"query": query})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestGraphQL_ValidQuery(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id title } }`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid query, got: %s", res.Error())
	}
}

func TestGraphQL_UnknownField(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { id unknownField } }`)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for unknown field 'unknownField'")
	}
}

func TestGraphQL_SyntaxError(t *testing.T) {
	v := newValidator(t)
	req := gqlRequest(`{ books { `)
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for syntax error")
	}
}

func TestGraphQL_EmptyQuery(t *testing.T) {
	v := newValidator(t)
	body, _ := json.Marshal(map[string]string{"query": ""})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := v.ValidateRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Error("expected invalid for empty query")
	}
}

func TestGraphQL_ValidResponse(t *testing.T) {
	v := newValidator(t)
	respBody, _ := json.Marshal(map[string]any{
		"data": map[string]any{"books": []any{}},
	})
	req := gqlRequest(`{ books { id } }`)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(respBody)),
		Request:    req,
	}
	res, err := v.ValidateResponse(resp, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Errorf("expected valid response, got: %s", res.Error())
	}
}
