module github.com/msgmate-io/rest-api-tool-integration

go 1.25.10

require (
	github.com/google/uuid v1.6.0
	github.com/msgmate-io/go-integration-interface v0.0.0
	github.com/msgmate-io/go-tool-interface v0.0.0
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/gorm v1.25.12
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/urfave/cli/v3 v3.11.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/msgmate-io/go-integration-interface => ../../go_integration_interface

replace github.com/msgmate-io/go-tool-interface => ../../go_tool_interface
