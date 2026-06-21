// Package all imports all schema format implementations so that a single
// blank import registers every parser:
//
//	import _ "github.com/simonwistow/apivalidator/all"
//
// Individual schema packages can also be imported selectively to avoid
// pulling in dependencies for formats you don't use.
package all

import (
	_ "github.com/simonwistow/apivalidator/schemas/capnproto" // Cap'n Proto
	_ "github.com/simonwistow/apivalidator/schemas/graphql"   // GraphQL SDL
	_ "github.com/simonwistow/apivalidator/schemas/grpc"      // gRPC / Protocol Buffers
	_ "github.com/simonwistow/apivalidator/schemas/jsonschema" // JSON Schema
	_ "github.com/simonwistow/apivalidator/schemas/openapi"   // OpenAPI 3.x + Swagger 2.0
	_ "github.com/simonwistow/apivalidator/schemas/raml"      // RAML 1.0
	_ "github.com/simonwistow/apivalidator/schemas/thrift"    // Apache Thrift
)
