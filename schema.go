package apivalidator

// Format identifies the description language a Schema was parsed from.
type Format int

const (
	// FormatOpenAPI3 selects the OpenAPI 3.x parser (versions 3.0, 3.1, and 3.2).
	FormatOpenAPI3 Format = iota
	// FormatOpenAPI2 selects the Swagger 2.0 parser.
	FormatOpenAPI2
	// FormatRAML selects the RAML 1.0 parser.
	FormatRAML

	// FormatGRPC selects the Protocol Buffers / gRPC parser (.proto text format).
	FormatGRPC
	// FormatCapnProto selects the Cap'n Proto parser (.capnp files).
	FormatCapnProto
	// FormatThrift selects the Apache Thrift IDL parser (.thrift files).
	FormatThrift

	// FormatJSONSchema selects the JSON Schema parser (draft-07, 2019-09, 2020-12).
	FormatJSONSchema
	// FormatGraphQL selects the GraphQL SDL parser (.graphql and .gql files).
	FormatGraphQL
)

func (f Format) String() string {
	switch f {
	case FormatOpenAPI3:
		return "openapi3"
	case FormatOpenAPI2:
		return "openapi2"
	case FormatRAML:
		return "raml"
	case FormatGRPC:
		return "grpc"
	case FormatCapnProto:
		return "capnproto"
	case FormatThrift:
		return "thrift"
	case FormatJSONSchema:
		return "jsonschema"
	case FormatGraphQL:
		return "graphql"
	default:
		return "unknown"
	}
}

// Schema is a parsed, ready-to-validate API description.
// Implementations are format-specific and opaque to callers.
type Schema interface {
	// Format returns the description language this schema was parsed from.
	Format() Format

	// Name returns a human-readable identifier for the schema — typically the
	// API title from the schema document, or the filename if no title exists.
	Name() string

	// Fingerprint returns a stable hash of the schema content. Two schemas with
	// the same fingerprint are guaranteed to produce the same validation results.
	Fingerprint() string
}

// ParseSchema parses a schema from raw bytes. The format must be specified
// explicitly — auto-detection across all formats is unreliable (JSON Schema and
// OpenAPI are both JSON/YAML; proto and capnp look similar).
//
// The returned Schema is safe for concurrent use and can be registered with
// multiple Validators.
func ParseSchema(format Format, data []byte) (Schema, error) {
	return parseSchema(format, data, "")
}

// ParseSchemaString parses a schema from a string.
func ParseSchemaString(format Format, s string) (Schema, error) {
	return parseSchema(format, []byte(s), "")
}

// ParseSchemaFile parses a schema from a file on disk.
// The file name is used as the schema name when no title is present in the
// schema document itself.
func ParseSchemaFile(format Format, path string) (Schema, error) {
	return parseSchemaFile(format, path)
}
