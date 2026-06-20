module github.com/simonwistow/apivalidator

go 1.25

require (
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/simonwistow/apianalyzer v0.0.0
	github.com/vektah/gqlparser/v2 v2.5.35
)

require (
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/simonwistow/apianalyzer => ../apianalyzer
