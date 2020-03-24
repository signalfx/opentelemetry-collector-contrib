module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8s

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector v0.2.6
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.4.0
	github.com/uber-go/atomic v1.4.0
	go.uber.org/zap v1.13.0
	k8s.io/api v0.0.0-20190813020757-36bff7324fb7
	k8s.io/apimachinery v0.0.0-20190809020650-423f5d784010
	k8s.io/client-go v12.0.0+incompatible
)

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab

replace github.com/open-telemetry/opentelemetry-collector => ../../../../opentelemetry-collector

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../
