// Copyright 2019, OpenTelemetry Authors
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

package carbonreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/config/configcheck"
	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

type mockMetricsConsumer struct {
}

var _ (consumer.MetricsConsumerOld) = (*mockMetricsConsumer)(nil)

func (m *mockMetricsConsumer) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	return nil
}

func TestCreateReceiver(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0" // Endpoint is required, not going to be used here.

	tReceiver, err := factory.CreateMetricsReceiver(zap.NewNop(), cfg, &mockMetricsConsumer{})
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	tReceiver, err = factory.CreateMetricsReceiver(zap.NewNop(), cfg, &mockMetricsConsumer{})
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)
}
