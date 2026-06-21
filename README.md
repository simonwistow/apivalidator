# apivalidator

A Go library for validating HTTP requests and responses against API schemas.
Supports eight schema formats — REST, RPC, and body-level — in a single, uniform API.
No code generation required; schemas are parsed at runtime.

```
go get github.com/simonwistow/apivalidator
```

## Supported formats

| Constant | Format | File types | Status |
|---|---|---|---|
| `FormatOpenAPI3` | OpenAPI 3.x (3.0 / 3.1 / 3.2) | `.yaml`, `.json` | Stable |
| `FormatOpenAPI2` | Swagger 2.0 | `.yaml`, `.json` | Stable |
| `FormatGRPC` | Protocol Buffers / gRPC | `.proto` | Stable |
| `FormatThrift` | Apache Thrift | `.thrift` | Stable |
| `FormatCapnProto` | Cap'n Proto | `.capnp` | Stable |
| `FormatGraphQL` | GraphQL SDL | `.graphql`, `.gql` | Stable |
| `FormatJSONSchema` | JSON Schema (draft-07 / 2019-09 / 2020-12) | `.json` | Stable |
| `FormatRAML` | RAML 1.0 | `.raml` | Stable |

## Quick start

```go
import (
    apivalidator "github.com/simonwistow/apivalidator"
    _ "github.com/simonwistow/apivalidator/schemas/openapi" // register OpenAPI parser
)

v, err := apivalidator.NewValidator().AddSchemaFile(apivalidator.FormatOpenAPI3, "api.yaml")
if err != nil {
    log.Fatal(err)
}

result, err := v.ValidateRequest(req)
if err == apivalidator.ErrNoMatchingSchema {
    // endpoint not covered by any schema
}
if !result.Valid {
    for _, v := range result.Violations {
        fmt.Println(v) // [error] /body/name: missing required field (missing_field)
    }
}
```

## Loading schemas

All three methods return `*Validator` so calls can be chained:

```go
// From a file (name inferred from filename or schema title)
v, err := apivalidator.NewValidator().AddSchemaFile(apivalidator.FormatOpenAPI3, "api.yaml")

// From a string
v, err := apivalidator.NewValidator().AddSchemaString(apivalidator.FormatJSONSchema, schemaJSON)

// From bytes
v, err := apivalidator.NewValidator().AddSchemaBytes(apivalidator.FormatGRPC, protoBytes)

// Pre-parsed schema (safe to share across validators)
schema, err := apivalidator.ParseSchemaFile(apivalidator.FormatGraphQL, "schema.graphql")
v := apivalidator.NewValidator().AddSchema(schema)
```

Import only the format packages you need, or import `all` to register every format:

```go
import _ "github.com/simonwistow/apivalidator/all" // registers all parsers
```

## Validating requests and responses

```go
// Validate an incoming request
result, err := v.ValidateRequest(req)

// Validate a response (body is read, buffered, and replaced on resp.Body)
result, err := v.ValidateResponse(resp, req)

// Validate a raw response body without the original request
// (useful for log replay / traffic capture scenarios)
result, err := v.ValidateResponseBody(body, "application/json")
```

`ValidationResult` fields:

```go
result.Valid            // bool
result.Violations       // []Violation — details of each failure
result.MatchedSchema    // schema name that was selected (or "multiple")
result.MatchedEndpoint  // path template that matched, e.g. "/users/{id}"
result.Error()          // human-readable summary of all violations
```

Each `Violation`:

```go
v.Code     // ViolationCode — stable machine-readable string, e.g. "invalid_body"
v.Severity // SeverityError | SeverityWarning | SeverityInfo
v.Path     // JSON Pointer to the failing value, e.g. "/body/email"
v.Message  // human-readable description
v.String() // "[error] /body/email: must be a valid email address (invalid_body)"
```

## Transport routing

The validator inspects `Content-Type` and path to route each request to the correct schema type automatically — no caller configuration needed.

| Content-Type | Path heuristic | Schema types selected |
|---|---|---|
| `application/grpc` | — | gRPC / Protobuf |
| `application/grpc-web[+proto]` | — | gRPC / Protobuf |
| `application/x-thrift` | — | Thrift |
| `application/graphql` | — | GraphQL |
| anything | ends with `/graphql` | GraphQL |
| anything else | — | OpenAPI, RAML, JSON Schema, Cap'n Proto |

## HTTP middleware

`Middleware` wraps any `http.Handler` to validate requests inline:

```go
import apivalidator "github.com/simonwistow/apivalidator"

handler = apivalidator.Middleware(v)(handler)
```

Default behaviour:
- **Valid request** — next handler is called normally.
- **Validation failure** — `400 application/json` response with violation details; next handler is NOT called.
- **`ErrNoMatchingSchema`** — request passes through unchanged (endpoint not covered).
- **`ErrBodyTooLarge`** — `400` plain-text error.

Custom rejection handler:

```go
handler = apivalidator.Middleware(v,
    apivalidator.WithOnViolation(func(w http.ResponseWriter, r *http.Request, result *apivalidator.ValidationResult) {
        w.WriteHeader(http.StatusUnprocessableEntity)
        json.NewEncoder(w).Encode(result.Violations)
    }),
)(handler)
```

## Multi-schema migration

Register multiple schema versions and use `PolicyAny` to accept requests valid under any one of them — useful when clients migrate independently:

```go
v := apivalidator.NewValidator(
    apivalidator.WithMultiSchemaPolicy(apivalidator.PolicyAny),
)
v.AddSchemaString(apivalidator.FormatJSONSchema, schemaV1)
v.AddSchemaString(apivalidator.FormatJSONSchema, schemaV2)

// A request valid under v1 OR v2 passes; only requests invalid under both fail.
result, err := v.ValidateRequest(req)
```

`PolicyAll` (the default) requires every registered matching schema to pass, which is stricter — useful for contract testing.

## Configuration options

Pass options to `NewValidator`:

| Option | Default | Description |
|---|---|---|
| `WithStrictMode(true)` | `false` | Unknown endpoints return a `CodeUnknownEndpoint` violation instead of `ErrNoMatchingSchema` |
| `WithIgnoreUnknownEndpoints()` | off | Unknown endpoints pass silently (useful for API gateways with catch-all routes) |
| `WithMultiSchemaPolicy(p)` | `PolicyAll` | `PolicyAll` or `PolicyAny` — see multi-schema section |
| `WithMaxBodyBytes(n)` | 4 MiB | Maximum body size buffered during validation |
| `WithValidateBody(false)` | `true` | Skip body parsing entirely (path/header-only validation) |
| `WithValidateQueryParams(false)` | `true` | Skip query parameter validation |
| `WithValidateHeaders(true)` | `false` | Enable header validation (most schemas do not declare headers) |
| `WithJSONSchemaScope(s)` | `JSONSchemaBoth` | Restrict JSON Schema to `JSONSchemaRequest` or `JSONSchemaResponse` only |
| `WithLogger(l)` | nil (silent) | `*slog.Logger` for diagnostic output |

## Error sentinel values

```go
apivalidator.ErrNoMatchingSchema // no registered schema covers this endpoint
apivalidator.ErrBodyTooLarge     // body exceeds WithMaxBodyBytes limit
apivalidator.ErrSchemaParseFailure // schema could not be parsed (wraps the underlying error)
```

## Examples

Runnable examples live in [examples/](examples/):

| Directory | Demonstrates |
|---|---|
| [examples/basic](examples/basic/) | Load a schema, validate a request, inspect violations |
| [examples/middleware](examples/middleware/) | HTTP middleware integration |
| [examples/migration](examples/migration/) | PolicyAny with two schema versions side-by-side |
| [examples/grpc](examples/grpc/) | gRPC / Protocol Buffers validation |
| [examples/graphql](examples/graphql/) | GraphQL request and response validation |
| [examples/log_replay](examples/log_replay/) | Offline response-body validation from captured traffic |
| [examples/multi_format](examples/multi_format/) | OpenAPI + gRPC + GraphQL in one validator |

Run any example:

```
go run ./examples/basic
```

## Thread safety

`Validator` is safe for concurrent use after construction. `AddSchema*` methods are also safe to call concurrently (they hold a write lock). `Schema` values returned by `ParseSchema*` are immutable and safe to share across multiple validators.
