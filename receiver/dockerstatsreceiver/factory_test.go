// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerstatsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	assert.Equal(t, "docker_stats", string(factory.Type()))

	config := factory.CreateDefaultConfig()
	assert.NotNil(t, config, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(config))
}

func TestCreateReceiver(t *testing.T) {
	factory := Factory{}
	config := factory.CreateDefaultConfig()

	traceReceiver, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), config, nil)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, traceReceiver)

	metricReceiver, err := factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), config, nil)
	assert.NoError(t, err, "Metric receiver creation failed")
	assert.NotNil(t, metricReceiver, "Receiver creation failed")
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := Factory{}
	config := factory.CreateDefaultConfig()
	receiverCfg := config.(*Config)

	receiverCfg.Endpoint = ""
	receiver, err := factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), receiverCfg, nil)
	assert.Nil(t, receiver)
	assert.Error(t, err)
	assert.Equal(t, "config.Endpoint must be specified", err.Error())

	receiverCfg.Endpoint = "\a"
	receiver, err = factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), receiverCfg, nil)
	assert.Nil(t, receiver)
	assert.Error(t, err)
	assert.Equal(
		t, "could not determine receiver transport: parse \"\\a\": net/url: invalid control character in URL",
		err.Error(),
	)
}
