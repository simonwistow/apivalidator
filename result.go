package apivalidator

import (
	"fmt"
	"strings"
)

// ValidationResult carries structured diagnostics from a single validation attempt.
type ValidationResult struct {
	Valid bool

	Violations []Violation

	// MatchedSchema identifies which registered schema was selected.
	// Set to "multiple" when more than one schema matched (PolicyAll/PolicyAny).
	MatchedSchema string

	// MatchedEndpoint is the path template that matched the request, e.g. "/users/{userID}".
	// Empty for non-REST schemas (gRPC, GraphQL, etc.).
	MatchedEndpoint string
}

// IsValid reports whether the result is valid. Equivalent to checking .Valid directly.
func (r *ValidationResult) IsValid() bool { return r.Valid }

// Error returns a human-readable summary of violations. It satisfies the error
// interface so callers that only check (ok, err) still get a useful message.
func (r *ValidationResult) Error() string {
	if r.Valid {
		return ""
	}
	if len(r.Violations) == 0 {
		return "validation failed (no violations recorded)"
	}
	msgs := make([]string, 0, len(r.Violations))
	for _, v := range r.Violations {
		msgs = append(msgs, v.String())
	}
	return strings.Join(msgs, "; ")
}

// Violation describes a single point of non-conformance.
type Violation struct {
	// Path is a JSON Pointer (RFC 6901) or field path to the failing value.
	// Empty string means the request or response as a whole failed.
	Path string

	// Message is a human-readable description of why this is a violation.
	Message string

	// Code is a stable, machine-readable identifier for the violation type.
	Code ViolationCode

	// Severity indicates how serious the violation is.
	Severity Severity
}

func (v Violation) String() string {
	if v.Path == "" {
		return fmt.Sprintf("[%s] %s (%s)", v.Severity, v.Message, v.Code)
	}
	return fmt.Sprintf("[%s] %s: %s (%s)", v.Severity, v.Path, v.Message, v.Code)
}

// ViolationCode is a stable, machine-readable identifier for a violation type.
// Values are defined as constants below and will not change between releases.
type ViolationCode string

// ViolationCode constants identify specific violation types. Values are stable
// across releases and safe to store or compare as strings.
const (
	CodeUnknownEndpoint    ViolationCode = "unknown_endpoint"
	CodeUnknownMethod      ViolationCode = "unknown_method"
	CodeInvalidPathParam   ViolationCode = "invalid_path_param"
	CodeMissingQueryParam  ViolationCode = "missing_query_param"
	CodeUnknownQueryParam  ViolationCode = "unknown_query_param"
	CodeInvalidQueryParam  ViolationCode = "invalid_query_param"
	CodeInvalidContentType ViolationCode = "invalid_content_type"
	CodeInvalidBody        ViolationCode = "invalid_body"
	CodeUnknownStatusCode  ViolationCode = "unknown_status_code"
	CodeInvalidResponse    ViolationCode = "invalid_response"
	CodeUnknownField       ViolationCode = "unknown_field"
	CodeMissingField       ViolationCode = "missing_field"
	CodeInvalidFieldType   ViolationCode = "invalid_field_type"
	CodeUnknownOperation   ViolationCode = "unknown_operation"
	CodeInvalidVariable    ViolationCode = "invalid_variable"
)

// Severity indicates how serious a violation is.
type Severity int

// Severity constants rank how serious a violation is.
const (
	SeverityError   Severity = iota // the request/response is definitively wrong
	SeverityWarning                 // may be wrong, or the schema is ambiguous
	SeverityInfo                    // informational only; does not affect Valid
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}
