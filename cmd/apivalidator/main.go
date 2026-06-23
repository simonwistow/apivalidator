// Command apivalidator validates HTTP requests and responses captured in HAR
// files against API schemas (OpenAPI, gRPC, GraphQL, Thrift, CapnProto, JSON Schema).
//
// Usage:
//
//	apivalidator [flags] <schema-file> [request.har ...]
//
// Flags:
//
//	-format    Schema format: openapi3, openapi2, grpc, graphql, thrift, capnproto, jsonschema
//	-strict    Treat unknown endpoints as errors (default: false)
//	-response  Also validate response bodies (default: false)
//	-q         Suppress progress output; only print violations
package main

import (
	"flag"
	"fmt"
	"os"

	apivalidator "github.com/simonwistow/apivalidator"
	_ "github.com/simonwistow/apivalidator/all"
)

func main() {
	formatFlag := flag.String("format", "", "schema format: openapi3, openapi2, grpc, graphql, thrift, capnproto, jsonschema")
	strictFlag := flag.Bool("strict", false, "treat unknown endpoints as errors")
	responseFlag := flag.Bool("response", false, "also validate response bodies")
	quietFlag := flag.Bool("q", false, "suppress progress output; only print violations")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: apivalidator [flags] <schema-file> [request.har ...]")
		fmt.Fprintln(os.Stderr, "       apivalidator -help")
		os.Exit(1)
	}

	format, err := parseFormat(*formatFlag)
	if err != nil {
		fatalf("error: %v\n", err)
	}

	schemaFile := args[0]
	inputFiles := args[1:]

	v := apivalidator.NewValidator(
		apivalidator.WithStrictMode(*strictFlag),
	)
	if _, err := v.AddSchemaFile(format, schemaFile); err != nil {
		fatalf("error loading schema: %v\n", err)
	}
	if !*quietFlag {
		fmt.Fprintf(os.Stderr, "loaded: %s\n", schemaFile)
	}

	if len(inputFiles) == 0 {
		fmt.Fprintln(os.Stderr, "no HAR files specified; nothing to validate")
		os.Exit(0)
	}

	exitCode := 0
	totalRequests := 0
	totalViolations := 0

	for _, harPath := range inputFiles {
		har, err := loadHAR(harPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", harPath, err)
			exitCode = 1
			continue
		}
		if !*quietFlag {
			fmt.Fprintf(os.Stderr, "replaying %d entries from %s\n", len(har.Log.Entries), harPath)
		}

		for i := range har.Log.Entries {
			entry := &har.Log.Entries[i]
			req, err := entry.Request.toHTTPRequest()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  entry %d: build request: %v\n", i+1, err)
				continue
			}
			totalRequests++

			result, err := v.ValidateRequest(req)
			if err == apivalidator.ErrNoMatchingSchema {
				if !*quietFlag {
					fmt.Printf("  [%d] %s %s → no matching schema\n", i+1, req.Method, req.URL.Path)
				}
				continue
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "  [%d] %s %s → validation error: %v\n", i+1, req.Method, req.URL.Path, err)
				exitCode = 1
				continue
			}

			printResult(i+1, req.Method, req.URL.Path, result, *quietFlag)
			totalViolations += len(result.Violations)
			if !result.Valid {
				exitCode = 1
			}

			if *responseFlag {
				resp := entry.Response.toHTTPResponse(req)
				rResult, rErr := v.ValidateResponse(resp, req)
				_ = resp.Body.Close()
				if rErr != nil && rErr != apivalidator.ErrNoMatchingSchema {
					fmt.Fprintf(os.Stderr, "  [%d] response validation error: %v\n", i+1, rErr)
				} else if rResult != nil {
					printResult(i+1, "RESPONSE "+req.Method, req.URL.Path, rResult, *quietFlag)
					totalViolations += len(rResult.Violations)
					if !rResult.Valid {
						exitCode = 1
					}
				}
			}
		}
	}

	if !*quietFlag {
		fmt.Fprintf(os.Stderr, "\nvalidated %d requests, %d violations\n", totalRequests, totalViolations)
	}
	os.Exit(exitCode)
}

func printResult(n int, method, path string, result *apivalidator.ValidationResult, quiet bool) {
	status := "✓"
	if !result.Valid {
		status = "✗"
	}
	if !quiet || !result.Valid {
		fmt.Printf("  [%d] %s %s %s → %s\n", n, status, method, path, result.MatchedEndpoint)
	}
	for _, v := range result.Violations {
		fmt.Printf("      %s\n", v)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
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
