// Package grpc implements validation for gRPC services described by .proto files.
// No code generation is required — schemas are parsed at runtime using
// jhump/protoreflect's protoparse package.
//
// Supports both raw gRPC (HTTP/2, Content-Type: application/grpc) and
// gRPC-Web (HTTP/1.1, Content-Type: application/grpc-web[+proto]).
// Transport is auto-detected from Content-Type; no caller flag needed.
package grpc

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatGRPC, parseSchema)
}

type grpcSchema struct {
	name        string
	fingerprint string
	file        *desc.FileDescriptor
}

func parseSchema(data []byte, hint string) (apivalidator.Schema, error) {
	filename := hint
	if filename == "" {
		filename = "service.proto"
	}

	p := protoparse.Parser{
		Accessor: func(f string) (io.ReadCloser, error) {
			if f == filename {
				return io.NopCloser(strings.NewReader(string(data))), nil
			}
			return nil, fmt.Errorf("%s: not found", f)
		},
	}
	fds, err := p.ParseFiles(filename)
	if err != nil {
		return nil, fmt.Errorf("parse .proto: %w", err)
	}
	if len(fds) == 0 {
		return nil, fmt.Errorf("no file descriptors parsed from %s", filename)
	}

	name := hint
	if name == "" {
		name = "gRPC"
	}
	sum := sha256.Sum256(data)
	return &grpcSchema{
		name:        name,
		fingerprint: hex.EncodeToString(sum[:]),
		file:        fds[0],
	}, nil
}

func (s *grpcSchema) Format() apivalidator.Format { return apivalidator.FormatGRPC }
func (s *grpcSchema) Name() string                { return s.name }
func (s *grpcSchema) Fingerprint() string         { return s.fingerprint }

func (s *grpcSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	svc, method, err := parseGRPCPath(req.URL.Path)
	if err != nil {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	md, found := s.findMethod(svc, method)
	if !found {
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
	if len(body) == 0 {
		res.Valid = true
		return res, nil
	}

	msgBytes, decErr := decodeGRPCFrame(body, isGRPCWeb(req))
	if decErr != nil {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("invalid gRPC frame: %v", decErr),
		}}
		return res, nil
	}

	violations := validateProtoMessage(msgBytes, md.GetInputType())
	res.Valid = len(violations) == 0
	res.Violations = violations
	return res, nil
}

func (s *grpcSchema) ValidateResponse(resp *http.Response, req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	svc, method, err := parseGRPCPath(req.URL.Path)
	if err != nil {
		return nil, apivalidator.ErrNoMatchingSchema
	}

	md, found := s.findMethod(svc, method)
	if !found {
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

	body, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		res.Valid = true
		return res, nil
	}

	msgBytes, decErr := decodeGRPCFrame(body, isGRPCWebCT(resp.Header.Get("Content-Type")))
	if decErr != nil {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("invalid gRPC frame: %v", decErr),
		}}
		return res, nil
	}

	violations := validateProtoMessage(msgBytes, md.GetOutputType())
	res.Valid = len(violations) == 0
	res.Violations = violations
	return res, nil
}

// findMethod looks up a service/method pair in the parsed .proto file.
// Matching is case-insensitive and ignores package prefixes in the service name.
func (s *grpcSchema) findMethod(svcName, methodName string) (*desc.MethodDescriptor, bool) {
	for _, svc := range s.file.GetServices() {
		if strings.EqualFold(svc.GetName(), svcName) ||
			strings.HasSuffix(strings.ToLower(svc.GetFullyQualifiedName()), "."+strings.ToLower(svcName)) {
			for _, m := range svc.GetMethods() {
				if strings.EqualFold(m.GetName(), methodName) {
					return m, true
				}
			}
		}
	}
	return nil, false
}

// parseGRPCPath parses /package.Service/Method → ("Service", "Method", nil).
func parseGRPCPath(path string) (svc, method string, err error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("not a gRPC path: %s", path)
	}
	// parts[0] may be "package.Service"; take the last segment.
	svcParts := strings.Split(parts[0], ".")
	return svcParts[len(svcParts)-1], parts[1], nil
}

// isGRPCWeb reports whether the request uses gRPC-Web transport.
func isGRPCWeb(req *http.Request) bool {
	return isGRPCWebCT(req.Header.Get("Content-Type"))
}

// isGRPCWebCT reports whether a Content-Type value indicates gRPC-Web.
func isGRPCWebCT(ct string) bool {
	return strings.HasPrefix(strings.ToLower(ct), "application/grpc-web")
}

// decodeGRPCFrame strips the 5-byte frame header and returns the protobuf bytes.
//
// Frame format (identical for raw gRPC and gRPC-Web data frames):
//
//	byte 0:   compression flag (0 = not compressed)
//	bytes 1-4: big-endian message length
//	bytes 5+:  protobuf message
//
// gRPC-Web trailer frames have byte 0 = 0x80; they are silently skipped.
func decodeGRPCFrame(data []byte, isWeb bool) ([]byte, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("frame too short: %d bytes", len(data))
	}
	flag := data[0]
	if isWeb && flag == 0x80 {
		// gRPC-Web trailer frame — skip; nothing to validate.
		return nil, nil
	}
	if flag != 0x00 {
		return nil, fmt.Errorf("compressed gRPC messages are not supported (flag=0x%02x)", flag)
	}
	msgLen := binary.BigEndian.Uint32(data[1:5])
	if int(msgLen) > len(data)-5 {
		return nil, fmt.Errorf("frame length %d exceeds available data %d", msgLen, len(data)-5)
	}
	return data[5 : 5+msgLen], nil
}

// validateProtoMessage decodes body as a protobuf message and checks required fields.
func validateProtoMessage(body []byte, msgDesc *desc.MessageDescriptor) []apivalidator.Violation {
	if len(body) == 0 || msgDesc == nil {
		return nil
	}

	msg := dynamic.NewMessage(msgDesc)
	if err := msg.Unmarshal(body); err != nil {
		return []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("protobuf decode error: %v", err),
		}}
	}

	// Check proto2 required fields. Proto3 fields are all optional by design.
	var out []apivalidator.Violation
	for _, fd := range msgDesc.GetFields() {
		if fd.IsRequired() {
			if msg.GetField(fd) == nil {
				out = append(out, apivalidator.Violation{
					Path:     fd.GetName(),
					Code:     apivalidator.CodeMissingField,
					Severity: apivalidator.SeverityError,
					Message:  fmt.Sprintf("required field %q is missing", fd.GetName()),
				})
			}
		}
	}
	return out
}
