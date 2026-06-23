package thrift

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// thriftIDL holds the parsed contents of a .thrift file relevant for validation.
type thriftIDL struct {
	services  map[string]*thriftService
	structs   map[string]*thriftStruct
	namespace string
}

type thriftService struct {
	name    string
	methods map[string]*thriftMethod
}

type thriftMethod struct {
	name   string
	args   []*thriftField
	result string
}

type thriftField struct {
	id       int
	name     string
	typ      string
	required bool
}

type thriftStruct struct {
	name   string
	fields []*thriftField
}

// parseIDL parses a .thrift file into a thriftIDL, extracting service and
// struct definitions. It is a minimal recursive-descent parser that handles
// the subset of the Thrift IDL needed for request routing and field validation.
func parseIDL(src string) (*thriftIDL, error) {
	p := &idlParser{src: src, pos: 0}
	idl := &thriftIDL{
		services: make(map[string]*thriftService),
		structs:  make(map[string]*thriftStruct),
	}
	for !p.eof() {
		p.skipWhitespaceAndComments()
		if p.eof() {
			break
		}
		tok := p.readIdent()
		switch tok {
		case "namespace":
			ns := p.readUntilNewline()
			parts := strings.Fields(ns)
			if len(parts) >= 2 {
				idl.namespace = parts[1]
			}
		case "include":
			p.readUntilNewline()
		case "const", "typedef", "enum", "exception":
			p.skipBlock()
		case "struct", "union":
			s, err := p.parseStruct()
			if err != nil {
				return nil, fmt.Errorf("parse struct: %w", err)
			}
			idl.structs[s.name] = s
		case "service":
			svc, err := p.parseService()
			if err != nil {
				return nil, fmt.Errorf("parse service: %w", err)
			}
			idl.services[svc.name] = svc
		default:
			// Skip unknown top-level token.
			p.readUntilNewline()
		}
	}
	return idl, nil
}

// idlParser is a simple tokeniser/parser for .thrift files.
type idlParser struct {
	src string
	pos int
}

func (p *idlParser) eof() bool { return p.pos >= len(p.src) }

func (p *idlParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *idlParser) skipWhitespaceAndComments() {
	for !p.eof() {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
			continue
		}
		// Single-line comments: # or //
		if c == '#' || (c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/') {
			for !p.eof() && p.src[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		// Block comment: /* ... */
		if c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '*' {
			p.pos += 2
			for !p.eof() {
				if p.src[p.pos] == '*' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/' {
					p.pos += 2
					break
				}
				p.pos++
			}
			continue
		}
		break
	}
}

func (p *idlParser) readIdent() string {
	p.skipWhitespaceAndComments()
	start := p.pos
	for !p.eof() {
		c := rune(p.src[p.pos])
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '.' {
			break
		}
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *idlParser) readUntilNewline() string {
	start := p.pos
	for !p.eof() && p.src[p.pos] != '\n' {
		p.pos++
	}
	return strings.TrimSpace(p.src[start:p.pos])
}

// skipBlock skips to end of a braced block at the same nesting level.
func (p *idlParser) skipBlock() {
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
		}
	}
}

func (p *idlParser) expect(ch byte) error {
	p.skipWhitespaceAndComments()
	if p.eof() || p.src[p.pos] != ch {
		got := "EOF"
		if !p.eof() {
			got = string(p.src[p.pos])
		}
		return fmt.Errorf("expected %q got %q", string(ch), got)
	}
	p.pos++
	return nil
}

func (p *idlParser) parseStruct() (*thriftStruct, error) {
	name := p.readIdent()
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	s := &thriftStruct{name: name}
	for {
		p.skipWhitespaceAndComments()
		if p.eof() || p.peek() == '}' {
			break
		}
		f, err := p.parseField()
		if err != nil {
			return nil, err
		}
		if f != nil {
			s.fields = append(s.fields, f)
		}
	}
	if err := p.expect('}'); err != nil {
		return nil, err
	}
	return s, nil
}

func (p *idlParser) parseField() (*thriftField, error) {
	p.skipWhitespaceAndComments()
	// Optional field ID: "1:"
	id := 0
	start := p.pos
	numStr := ""
	for !p.eof() && (p.src[p.pos] >= '0' && p.src[p.pos] <= '9') {
		numStr += string(p.src[p.pos])
		p.pos++
	}
	p.skipWhitespaceAndComments()
	if !p.eof() && p.src[p.pos] == ':' {
		p.pos++
		if numStr != "" {
			if v, err := strconv.Atoi(numStr); err == nil {
				id = v
			}
		}
	} else {
		// Not a field ID — rewind.
		p.pos = start
	}

	// Optional: required/optional keyword
	required := false
	p.skipWhitespaceAndComments()
	maybeKw := p.readIdent()
	typStr := ""
	switch maybeKw {
	case "required":
		required = true
		typStr = p.readIdent()
	case "optional":
		typStr = p.readIdent()
	default:
		typStr = maybeKw
	}

	if typStr == "" {
		return nil, nil
	}

	// Handle generic types like map<k,v>, list<t>, set<t>
	p.skipWhitespaceAndComments()
	if !p.eof() && p.src[p.pos] == '<' {
		depth := 0
		for !p.eof() {
			c := p.src[p.pos]
			p.pos++
			if c == '<' {
				depth++
			} else if c == '>' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
	}

	name := p.readIdent()
	if name == "" {
		return nil, nil
	}

	// Skip optional default value and/or trailing comma/semicolon.
	p.skipWhitespaceAndComments()
	if !p.eof() && p.src[p.pos] == '=' {
		p.pos++
		p.readUntilNewline()
	}
	p.skipWhitespaceAndComments()
	if !p.eof() && (p.src[p.pos] == ',' || p.src[p.pos] == ';') {
		p.pos++
	}

	return &thriftField{id: id, name: name, typ: typStr, required: required}, nil
}

func (p *idlParser) parseService() (*thriftService, error) {
	name := p.readIdent()
	// Skip optional "extends Something"
	p.skipWhitespaceAndComments()
	if !p.eof() && p.src[p.pos] != '{' {
		p.readIdent() // "extends"
		p.readIdent() // parent service name
	}
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	svc := &thriftService{name: name, methods: make(map[string]*thriftMethod)}
	for {
		p.skipWhitespaceAndComments()
		if p.eof() || p.peek() == '}' {
			break
		}
		m, err := p.parseMethod()
		if err != nil {
			return nil, fmt.Errorf("parse method in %s: %w", name, err)
		}
		if m != nil {
			svc.methods[m.name] = m
		}
	}
	if err := p.expect('}'); err != nil {
		return nil, err
	}
	return svc, nil
}

func (p *idlParser) parseMethod() (*thriftMethod, error) {
	p.skipWhitespaceAndComments()
	if p.eof() || p.peek() == '}' {
		return nil, nil
	}
	// Optional: oneway keyword
	tok := p.readIdent()
	if tok == "oneway" {
		tok = p.readIdent()
	}
	returnType := tok
	name := p.readIdent()
	if name == "" {
		p.skipBlock()
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	var args []*thriftField
	for {
		p.skipWhitespaceAndComments()
		if p.eof() || p.peek() == ')' {
			break
		}
		f, err := p.parseField()
		if err != nil {
			return nil, err
		}
		if f != nil {
			args = append(args, f)
		}
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	// Skip optional throws clause — peek first so we don't consume the next method's return type.
	p.skipWhitespaceAndComments()
	saved := p.pos
	if !p.eof() {
		next := p.readIdent()
		if next == "throws" {
			p.expect('(') //nolint:errcheck,gosec
			depth := 1
			for !p.eof() && depth > 0 {
				c := p.src[p.pos]
				p.pos++
				if c == '(' {
					depth++
				} else if c == ')' {
					depth--
				}
			}
		} else {
			// Not a throws clause — rewind so the next parseMethod call sees the return type.
			p.pos = saved
		}
	}
	// Skip trailing comma/semicolon
	p.skipWhitespaceAndComments()
	if !p.eof() && (p.src[p.pos] == ',' || p.src[p.pos] == ';') {
		p.pos++
	}
	return &thriftMethod{name: name, args: args, result: returnType}, nil
}
