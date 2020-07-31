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
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Receivers[configmodels.Type(typeStr)] = factory
	config, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, 2, len(config.Receivers))

	defaultConfig := config.Receivers["docker_stats"]
	assert.Equal(t, factory.CreateDefaultConfig(), defaultConfig)

	dcfg := defaultConfig.(*Config)
	assert.Equal(t, "docker_stats", dcfg.Name())
	assert.Equal(t, "unix:///var/run/docker.sock", dcfg.Endpoint)
	assert.Equal(t, 10*time.Second, dcfg.CollectionInterval)
	assert.Equal(t, 5*time.Second, dcfg.Timeout)

	assert.Nil(t, dcfg.ExcludedImages)
	assert.Nil(t, dcfg.ContainerLabelsToMetricLabels)
	assert.Nil(t, dcfg.EnvVarsToMetricLabels)

	assert.False(t, dcfg.ProvideAllBlockIOMetrics)
	assert.False(t, dcfg.ProvideAllCPUMetrics)
	assert.False(t, dcfg.ProvideAllMemoryMetrics)
	assert.False(t, dcfg.ProvideAllNetworkMetrics)

	ascfg := config.Receivers["docker_stats/allsettings"].(*Config)
	assert.Equal(t, "docker_stats/allsettings", ascfg.Name())
	assert.Equal(t, "http://example.com/", ascfg.Endpoint)
	assert.Equal(t, 2*time.Second, ascfg.CollectionInterval)
	assert.Equal(t, 20*time.Second, ascfg.Timeout)

	assert.Equal(t, []string{
		"undesired-container",
		"another-*-container",
	}, ascfg.ExcludedImages)

	assert.Equal(t, map[string]string{
		"my.container.label":       "my-metric-label",
		"my.other.container.label": "my-other-metric-label",
	}, ascfg.ContainerLabelsToMetricLabels)

	assert.Equal(t, map[string]string{
		"my_environment_variable":       "my-metric-label",
		"my_other_environment_variable": "my-other-metric-label",
	}, ascfg.EnvVarsToMetricLabels)

	assert.True(t, ascfg.ProvideAllBlockIOMetrics)
	assert.True(t, ascfg.ProvideAllCPUMetrics)
	assert.True(t, ascfg.ProvideAllMemoryMetrics)
	assert.True(t, ascfg.ProvideAllNetworkMetrics)
}
