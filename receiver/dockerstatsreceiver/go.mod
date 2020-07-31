module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver

go 1.14

require (
	github.com/Azure/go-autorest v11.2.8+incompatible // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/docker/docker v17.12.0-ce-rc1.0.20200514230353-811a247d06e8+incompatible
	github.com/gobwas/glob v0.2.3
	github.com/golang/protobuf v1.4.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.0.0-20200518175917-05cf2ea24e6c
	github.com/ory/x v0.0.109 // indirect
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.7.0
	go.uber.org/zap v1.15.0
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver => ../redisreceiver
