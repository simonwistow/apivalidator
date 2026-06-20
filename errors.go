package apivalidator

import "errors"

// ErrNoMatchingSchema is returned by ValidateRequest and ValidateResponse when
// no registered schema covers the requested endpoint. It is returned as the
// error value, not as a Violation, so callers can distinguish "definitively
// invalid" from "not covered by any known schema."
//
// Use WithStrictMode(true) to promote this to a SeverityError Violation instead.
// Use WithIgnoreUnknownEndpoints() to suppress it entirely.
var ErrNoMatchingSchema = errors.New("no matching schema for this endpoint")

// ErrBodyTooLarge is returned when a request or response body exceeds the
// configured MaxBodyBytes limit.
var ErrBodyTooLarge = errors.New("body exceeds maximum buffering limit")

// ErrSchemaParseFailure is returned by ParseSchema when the schema document
// cannot be parsed. The error wraps the underlying parse error.
var ErrSchemaParseFailure = errors.New("schema parse failure")
