module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receiver_creator

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/jwangsadinata/go-multimap v0.0.0-20190620162914-c29f3d7f33b6
	github.com/open-telemetry/opentelemetry-collector v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.13.0
)

// Same version as from go.mod. Required to make go list -m work.
replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../../extension/observer
