package apivalidator

import (
	"fmt"
	"os"
	"path/filepath"
)

// parseSchema is the internal implementation of ParseSchema and ParseSchemaString.
// hint is an optional filename used as a fallback name when the schema has no title.
func parseSchema(format Format, data []byte, hint string) (Schema, error) {
	parser, ok := registeredParsers[format]
	if !ok {
		return nil, fmt.Errorf("%w: no parser registered for format %s", ErrSchemaParseFailure, format)
	}
	s, err := parser(data, hint)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaParseFailure, err)
	}
	return s, nil
}

// parseSchemaFile reads a file from disk and parses it. Formats that register
// a file-aware parser via RegisterFileParser are called directly so they can
// preserve the full path for include resolution.
func parseSchemaFile(format Format, path string) (Schema, error) {
	if fp, ok := registeredFileParsers[format]; ok {
		s, err := fp(path)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSchemaParseFailure, err)
		}
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrSchemaParseFailure, path, err)
	}
	return parseSchema(format, data, filepath.Base(path))
}

// parserFunc is the signature for format-specific schema parsers.
// hint is the filename (or empty string) used as a fallback schema name.
type parserFunc func(data []byte, hint string) (Schema, error)

// registeredParsers maps Format → parser. Sub-packages register via init().
var registeredParsers = map[Format]parserFunc{}

// RegisterParser is called by schema sub-packages in their init() functions
// to associate a Format with its parser implementation.
func RegisterParser(f Format, fn parserFunc) {
	registeredParsers[f] = fn
}

// parserFileFunc is the signature for format-specific file-aware parsers.
// Formats that support includes (e.g. RAML !include) register one of these
// so that ParseSchemaFile can pass the full path rather than just the bytes.
type parserFileFunc func(path string) (Schema, error)

// registeredFileParsers maps Format → file-aware parser.
var registeredFileParsers = map[Format]parserFileFunc{}

// RegisterFileParser registers a file-aware parser for formats that need the
// full path to resolve includes. When set, ParseSchemaFile calls this instead
// of reading the file and calling the standard byte-level parser.
func RegisterFileParser(f Format, fn parserFileFunc) {
	registeredFileParsers[f] = fn
}
