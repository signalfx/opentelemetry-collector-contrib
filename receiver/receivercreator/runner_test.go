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

package receivercreator

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func exampleReceiverCreator(t *testing.T, name string) *receiverCreator {
	cfg := exampleCreatorFactory(t)
	rcCfg := cfg.Receivers[name]
	factory := &Factory{}
	rc, err := factory.CreateMetricsReceiver(zap.NewNop(), rcCfg, &mockMetricsConsumer{})
	require.NoError(t, err)
	return rc.(*receiverCreator)
}

func Test_loadAndCreateRuntimeReceiver(t *testing.T) {
	rc := exampleReceiverCreator(t, "receiver_creator/1")
	subConfig := rc.cfg.subreceiverConfigs["examplereceiver/1"]
	require.NotNil(t, subConfig)
	loader := rc.newLoader(component.NewMockHost())
	loadedConfig, err := loader.loadRuntimeReceiverConfig(subConfig, map[string]interface{}{
		"endpoint": "localhost:12345",
	})
	require.NoError(t, err)
	assert.NotNil(t, loadedConfig)
	exampleConfig := loadedConfig.(*config.ExampleReceiver)
	// Verify that the overridden endpoint is used instead of the one in the config file.
	assert.Equal(t, "localhost:12345", exampleConfig.Endpoint)
	assert.Equal(t, "receiver_creator/1/examplereceiver/1{endpoint=\"localhost:12345\"}", exampleConfig.Name())

	// Test that metric receiver can be created from loaded config.
	t.Run("test create receiver from loaded config", func(t *testing.T) {
		recvr, err := loader.createRuntimeReceiver(loadedConfig)
		require.NoError(t, err)
		assert.NotNil(t, recvr)
		exampleReceiver := recvr.(*config.ExampleReceiverProducer)
		assert.Equal(t, rc.nextConsumer, exampleReceiver.MetricsConsumer)
	})
}

func TestStartStopReceiver(t *testing.T) {
	rc := exampleReceiverCreator(t, "receiver_creator/1")
	loader := rc.newLoader(component.NewMockHost())
	rcvr, err := loader.StartReceiver(rc.cfg.subreceiverConfigs["examplereceiver/1"], map[string]interface{}{})
	require.NotNil(t, rcvr)
	require.NoError(t, err)

	require.Len(t, loader.receivers, 1)

	t.Run("test stop receiver", func(t *testing.T) {
		assert.NoError(t, loader.StopReceiver(rcvr))
	})
}

func TestShutdownReceivers(t *testing.T) {
	rc := exampleReceiverCreator(t, "receiver_creator/1")
	loader := rc.newLoader(component.NewMockHost())
	_, err := loader.StartReceiver(rc.cfg.subreceiverConfigs["examplereceiver/1"], map[string]interface{}{})
	require.NoError(t, err)

	require.Len(t, loader.receivers, 1)
	assert.NoError(t, loader.Shutdown())
	assert.Nil(t, loader.receivers)
}
