module github.com/simonwistow/apivalidator

go 1.25

require (
	capnproto.org/go/capnp/v3 v3.1.0-alpha.2
	github.com/apache/thrift v0.23.0
	github.com/jhump/protoreflect v1.18.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/simonwistow/apianalyzer v0.0.0
	github.com/vektah/gqlparser/v2 v2.5.35
)

require (
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/colega/zeropool v0.0.0-20230505084239-6fb4a4f75381 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/petermattis/goid v0.0.0-20260113132338-7c7de50cc741 // indirect
	golang.org/x/sync v0.8.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/simonwistow/apianalyzer => ../apianalyzer
