// Package jsonschema implements body validation using JSON Schema documents.
// Supports drafts 07, 2019-09, and 2020-12 (auto-detected from the $schema keyword).
// When registered as a standalone schema (not embedded in OpenAPI), applies to
// both request and response bodies by default (configurable via WithJSONSchemaScope).
// Implemented in Phase 2.
package jsonschema
