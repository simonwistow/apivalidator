// Command apivalidator validates HTTP requests and responses captured in HAR
// files against API schemas (OpenAPI, gRPC, GraphQL, Thrift, CapnProto, JSON Schema).
//
// Usage:
//
//	apivalidator [flags] <schema-file> [request.har ...]
//
// Flags:
//
//	-format   Schema format: openapi3, openapi2, grpc, graphql, thrift, capnproto, jsonschema
//	-har      Input files are HAR (HTTP Archive) format (default: auto-detect)
//	-strict   Treat unknown endpoints as errors (default: warnings)
//	-q        Suppress progress output; only print violations
package main

import (
	"flag"
	"fmt"
	"os"

	_ "github.com/simonwistow/apivalidator/all"
	apivalidator "github.com/simonwistow/apivalidator"
)

func main() {
	formatFlag := flag.String("format", "", "schema format: openapi3, openapi2, grpc, graphql, thrift, capnproto, jsonschema")
	strictFlag := flag.Bool("strict", false, "treat unknown endpoints as errors")
	quietFlag  := flag.Bool("q", false, "suppress progress output")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: apivalidator [flags] <schema-file> [request.har ...]")
		fmt.Fprintln(os.Stderr, "       apivalidator -help")
		os.Exit(1)
	}

	format, err := parseFormat(*formatFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	schemaFile := args[0]
	inputFiles := args[1:]

	v := apivalidator.NewValidator(
		apivalidator.WithStrictMode(*strictFlag),
	)
	if _, err := v.AddSchemaFile(format, schemaFile); err != nil {
		fmt.Fprintf(os.Stderr, "error loading schema: %v\n", err)
		os.Exit(1)
	}
	if !*quietFlag {
		fmt.Fprintf(os.Stderr, "loaded schema: %s\n", schemaFile)
	}

	if len(inputFiles) == 0 {
		fmt.Fprintln(os.Stderr, "no input files specified; nothing to validate")
		os.Exit(0)
	}

	// Phase 6: HAR replay will be implemented here.
	_ = v
	_ = inputFiles
	fmt.Fprintln(os.Stderr, "HAR replay not yet implemented (Phase 6)")
	os.Exit(1)
}

func parseFormat(s string) (apivalidator.Format, error) {
	switch s {
	case "", "openapi3":
		return apivalidator.FormatOpenAPI3, nil
	case "openapi2":
		return apivalidator.FormatOpenAPI2, nil
	case "grpc":
		return apivalidator.FormatGRPC, nil
	case "graphql":
		return apivalidator.FormatGraphQL, nil
	case "thrift":
		return apivalidator.FormatThrift, nil
	case "capnproto":
		return apivalidator.FormatCapnProto, nil
	case "jsonschema":
		return apivalidator.FormatJSONSchema, nil
	default:
		return 0, fmt.Errorf("unknown format %q; valid values: openapi3, openapi2, grpc, graphql, thrift, capnproto, jsonschema", s)
	}
}
