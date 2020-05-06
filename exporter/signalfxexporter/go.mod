module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200503151053-5d1aacc0e168
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.0.0
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.0-20190530013331-054be550cb49
	github.com/stretchr/testify v1.5.1
	go.uber.org/zap v1.14.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver => ../../receiver/k8sclusterreceiver
