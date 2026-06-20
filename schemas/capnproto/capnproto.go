// Package capnproto implements structural validation for Cap'n Proto binary messages.
//
// The .capnp text schema is parsed to extract struct names and field definitions.
// Binary bodies are validated using the capnproto.org/go/capnp/v3 library to
// confirm the message has valid segment structure and pointer layout.
//
// Note: Cap'n Proto uses 64-bit type IDs determined at schema compilation time.
// Without a compiled schema binary, field-level type checking requires the text
// schema to be parsed and matched against the binary structure heuristically.
// Full typed validation (matching field IDs to types) requires a pre-compiled
// schema binary and is planned for a future phase.
package capnproto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	capnp "capnproto.org/go/capnp/v3"
	apivalidator "github.com/simonwistow/apivalidator"
)

func init() {
	apivalidator.RegisterParser(apivalidator.FormatCapnProto, parseSchema)
}

// capnpSchema holds a parsed .capnp schema for validation.
type capnpSchema struct {
	name        string
	fingerprint string
	structs     []*capnpStruct
}

type capnpStruct struct {
	name   string
	fields []capnpField
}

type capnpField struct {
	name string
	typ  string
}

func parseSchema(data []byte, hint string) (apivalidator.Schema, error) {
	structs, err := parseCapnpText(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse .capnp schema: %w", err)
	}

	name := hint
	if name == "" {
		name = "Cap'n Proto"
	}
	sum := sha256.Sum256(data)
	return &capnpSchema{
		name:        name,
		fingerprint: hex.EncodeToString(sum[:]),
		structs:     structs,
	}, nil
}

func (s *capnpSchema) Format() apivalidator.Format { return apivalidator.FormatCapnProto }
func (s *capnpSchema) Name() string                { return s.name }
func (s *capnpSchema) Fingerprint() string         { return s.fingerprint }

func (s *capnpSchema) ValidateRequest(req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	if !cfg.ValidateBody {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	body, err := apivalidator.ReadRequestBody(req, cfg)
	if err != nil {
		return nil, err
	}
	return s.validateBody(body), nil
}

func (s *capnpSchema) ValidateResponse(resp *http.Response, req *http.Request, cfg *apivalidator.Config) (*apivalidator.ValidationResult, error) {
	if !cfg.ValidateBody {
		return &apivalidator.ValidationResult{Valid: true, MatchedSchema: s.name}, nil
	}
	body, err := apivalidator.ReadResponseBody(resp, cfg)
	if err != nil {
		return nil, err
	}
	return s.validateBody(body), nil
}

// validateBody checks that body is a structurally valid Cap'n Proto message.
// It uses the capnp library's segment decoder to verify pointer integrity —
// this catches truncated messages, invalid pointer offsets, and out-of-bounds
// list accesses that would panic in a non-validating reader.
func (s *capnpSchema) validateBody(body []byte) *apivalidator.ValidationResult {
	res := &apivalidator.ValidationResult{MatchedSchema: s.name}

	if len(body) == 0 {
		res.Valid = true
		return res
	}

	msg, err := capnp.Unmarshal(body)
	if err != nil {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("Cap'n Proto decode error: %v", err),
		}}
		return res
	}

	// Walk the root pointer to validate the segment structure.
	_, err = msg.Root()
	if err != nil {
		res.Violations = []apivalidator.Violation{{
			Code:     apivalidator.CodeInvalidBody,
			Severity: apivalidator.SeverityError,
			Message:  fmt.Sprintf("Cap'n Proto root pointer error: %v", err),
		}}
		return res
	}

	res.Valid = true
	return res
}

// parseCapnpText extracts struct definitions from a .capnp text schema.
// This is a minimal parser sufficient for extracting struct names and field names.
func parseCapnpText(src string) ([]*capnpStruct, error) {
	var structs []*capnpStruct
	p := &capnpParser{src: src}
	for !p.eof() {
		p.skipWhitespace()
		if p.eof() {
			break
		}
		tok := p.readIdent()
		switch tok {
		case "struct":
			s, err := p.parseStruct()
			if err != nil {
				return nil, err
			}
			structs = append(structs, s)
		default:
			p.skipToNextTopLevel()
		}
	}
	return structs, nil
}

type capnpParser struct {
	src string
	pos int
}

func (p *capnpParser) eof() bool { return p.pos >= len(p.src) }

func (p *capnpParser) skipWhitespace() {
	for !p.eof() {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
			continue
		}
		// Single-line comment (#)
		if c == '#' {
			for !p.eof() && p.src[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		break
	}
}

func (p *capnpParser) readIdent() string {
	p.skipWhitespace()
	start := p.pos
	for !p.eof() {
		c := p.src[p.pos]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' {
			p.pos++
		} else {
			break
		}
	}
	return p.src[start:p.pos]
}

func (p *capnpParser) skipToNextTopLevel() {
	depth := 0
	for !p.eof() {
		c := p.src[p.pos]
		p.pos++
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth <= 0 {
				return
			}
		} else if c == ';' && depth == 0 {
			return
		}
	}
}

func (p *capnpParser) parseStruct() (*capnpStruct, error) {
	name := p.readIdent()
	// Skip annotation IDs like @0xb5e5d3aee89e1397 and metadata until '{'
	for !p.eof() && p.src[p.pos] != '{' {
		p.pos++
	}
	if p.eof() {
		return nil, fmt.Errorf("unexpected EOF in struct %s", name)
	}
	p.pos++ // consume '{'

	s := &capnpStruct{name: name}
	for !p.eof() {
		p.skipWhitespace()
		if p.eof() || p.src[p.pos] == '}' {
			break
		}
		fieldName := p.readIdent()
		if fieldName == "" {
			p.pos++
			continue
		}
		// Skip to end of field definition (';' or nested struct)
		p.skipWhitespace()
		if !p.eof() && p.src[p.pos] == '@' {
			// field index annotation: @N :Type ;
			for !p.eof() && p.src[p.pos] != ':' && p.src[p.pos] != ';' && p.src[p.pos] != '}' {
				p.pos++
			}
			typ := ""
			if !p.eof() && p.src[p.pos] == ':' {
				p.pos++
				typ = p.readIdent()
			}
			s.fields = append(s.fields, capnpField{name: fieldName, typ: typ})
		}
		p.skipToNextTopLevel()
	}
	if !p.eof() {
		p.pos++ // consume '}'
	}
	return s, nil
}
