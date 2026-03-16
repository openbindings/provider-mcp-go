module github.com/openbindings/provider-mcp-go

go 1.23.0

toolchain go1.24.1

require (
	github.com/modelcontextprotocol/go-sdk v1.3.0
	github.com/openbindings/openbindings-go v0.0.0
)

require (
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
)

replace github.com/openbindings/openbindings-go => ../openbindings-go
