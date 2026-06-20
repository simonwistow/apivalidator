// Package all imports all schema format implementations so that a single
// blank import registers every parser:
//
//	import _ "github.com/simonwistow/apivalidator/all"
//
// Individual schema packages can also be imported selectively to avoid
// pulling in dependencies for formats you don't use.
package all

import (
	_ "github.com/simonwistow/apivalidator/schemas/openapi" // OpenAPI 3.x + Swagger 2.0
	// Phase 2: _ "github.com/simonwistow/apivalidator/schemas/jsonschema"
	// Phase 3: _ "github.com/simonwistow/apivalidator/schemas/graphql"
	// Phase 4: _ "github.com/simonwistow/apivalidator/schemas/grpc"
	// Phase 5: _ "github.com/simonwistow/apivalidator/schemas/capnproto"
	// Phase 5: _ "github.com/simonwistow/apivalidator/schemas/thrift"
)
