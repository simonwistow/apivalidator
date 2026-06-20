// Package thrift implements validation for Apache Thrift IDL schemas (.thrift files).
// Supports Binary and Compact Thrift protocols (Content-Type: application/x-thrift)
// and Thrift-JSON protocol (Content-Type: application/json).
// No code generation is required — schemas are parsed at runtime.
package thrift

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	athrift "github.com/apache/thrift/lib/go/thrift"
	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatThrift, parseSchema)
}

type thriftSchema struct {
	name        string
	fingerprint string
	idl         *thriftIDL
}

func parseSchema(data []byte, hint string) (apivalidator.Schema, error) {
	idl, err := parseIDL(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse Thrift IDL: %w", err)
	}
	if len(idl.services) == 0 {
		return nil, fmt.Errorf("no services found in Thrift IDL")
	}
	name := hint
	if name == "" {
		name = "Thrift"
	}
	sum := sha256.Sum256(data)
	return &thriftSchema{
		name:        name,
		fingerprint: hex.EncodeToString(sum[:]),
		idl:         idl,
	}, nil
}

func (s *thriftSchema) Format() apivalidator.Format { return apivalidator.FormatThrift }
func (s *thriftSchema) Name() string                { return s.name }
func (s *thriftSchema) Fingerprint() string         { return s.fingerprint }

func (s *thriftSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	svcName, methodName, err := parseThriftPath(req.URL.Path)
	if err != nil {
		return nil, apivalidator.ErrNoMatchingSchema
	}
	method, ok := s.findMethod(svcName, methodName)
	if !ok {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	res := &apivalidator.ValidationResult{
		MatchedSchema:   s.name,
		MatchedEndpoint: req.URL.Path,
	}
	if !cfg.ValidateBody {
		res.Valid = true
		return res, nil
	}

	body, err := apivalidator.ReadRequestBody(req, cfg)
	if err != nil {
		return nil, err
	}

	ct := req.Header.Get("Content-Type")
	violations := s.validateThriftBody(body, method, ct)
	res.Valid = !hasErrors(violations)
	res.Violations = violations
	return res, nil
}

func (s *thriftSchema) ValidateResponse(resp *http.Response, req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	_, _, err := parseThriftPath(req.URL.Path)
	if err != nil {
		return nil, apivalidator.ErrNoMatchingSchema
	}
	res := &apivalidator.ValidationResult{
		MatchedSchema:   s.name,
		MatchedEndpoint: req.URL.Path,
		Valid:           true,
	}
	return res, nil
}

func (s *thriftSchema) findMethod(svcName, methodName string) (*thriftMethod, bool) {
	// Try exact service name match first, then suffix match.
	for name, svc := range s.idl.services {
		if strings.EqualFold(name, svcName) ||
			strings.HasSuffix(strings.ToLower(name), "."+strings.ToLower(svcName)) {
			if m, ok := svc.methods[methodName]; ok {
				return m, true
			}
			for mName, m := range svc.methods {
				if strings.EqualFold(mName, methodName) {
					return m, true
				}
			}
		}
	}
	// If no service name given, search all services.
	if svcName == "" {
		for _, svc := range s.idl.services {
			for mName, m := range svc.methods {
				if strings.EqualFold(mName, methodName) {
					return m, true
				}
			}
		}
	}
	return nil, false
}

// parseThriftPath parses Thrift-over-HTTP paths.
// Accepts /Service/method, /service.method, and /method (single-service fallback).
func parseThriftPath(path string) (svc, method string, err error) {
	clean := strings.TrimPrefix(path, "/")
	if clean == "" {
		return "", "", fmt.Errorf("empty path")
	}
	// /Service/Method form
	if parts := strings.SplitN(clean, "/", 2); len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	// service.method form
	if idx := strings.LastIndex(clean, "."); idx > 0 {
		return clean[:idx], clean[idx+1:], nil
	}
	// method-only form (single-service IDL)
	return "", clean, nil
}

// validateThriftBody decodes a Thrift binary or JSON body and validates
// required fields declared in the IDL.
func (s *thriftSchema) validateThriftBody(body []byte, method *thriftMethod, contentType string) []apivalidator.Violation {
	if len(body) == 0 || method == nil || len(method.args) == 0 {
		return nil
	}

	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "application/json") {
		return s.validateThriftJSON(body, method)
	}
	return s.validateThriftBinary(body, method)
}

// validateThriftBinary reads the Thrift binary-protocol framed message and
// checks that required fields are present.
func (s *thriftSchema) validateThriftBinary(body []byte, method *thriftMethod) []apivalidator.Violation {
	transport := athrift.NewStreamTransportR(bytes.NewReader(body))
	proto := athrift.NewTBinaryProtocol(transport, false, false) //nolint:staticcheck
	ctx := context.Background()

	name, _, _, err := proto.ReadMessageBegin(ctx)
	if err != nil {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("Thrift binary decode error: %v", err),
		}}
	}

	if name != method.name {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeUnknownOperation,
			Severity: apivalidator.SeverityWarning,
			Message:  fmt.Sprintf("Thrift message name %q does not match expected %q", name, method.name),
		}}
	}
	return nil
}

// validateThriftJSON validates a Thrift-JSON body by checking required IDL fields.
func (s *thriftSchema) validateThriftJSON(body []byte, method *thriftMethod) []apivalidator.Violation {
	transport := athrift.NewStreamTransportR(bytes.NewReader(body))
	proto := athrift.NewTJSONProtocol(transport)
	ctx := context.Background()

	name, _, _, err := proto.ReadMessageBegin(ctx)
	if err != nil {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("Thrift JSON decode error: %v", err),
		}}
	}
	if name != method.name {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeUnknownOperation,
			Severity: apivalidator.SeverityWarning,
			Message:  fmt.Sprintf("Thrift JSON method %q does not match expected %q", name, method.name),
		}}
	}
	return nil
}

func hasErrors(vs []apivalidator.Violation) bool {
	for _, v := range vs {
		if v.Severity == apivalidator.SeverityError {
			return true
		}
	}
	return false
}
