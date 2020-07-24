// Copyright The OpenTelemetry Authors
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

// +build integration

package dockerstatsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/container"
)

type testHost struct {
	component.Host
	t *testing.T
}

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("Receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

func factory() (Factory, *Config) {
	f := Factory{}
	config := f.CreateDefaultConfig().(*Config)
	config.CollectionInterval = 1 * time.Second
	return f, config
}

func TestDefaultMetricsIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))

	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	consumer := &exportertest.SinkMetricsExporterOld{}
	f, config := factory()
	receiver, err := f.CreateMetricsReceiver(context.Background(), logger, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestAllMetricsIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))

	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	consumer := &exportertest.SinkMetricsExporterOld{}
	f, config := factory()

	config.ProvideAllBlockIOMetrics = true
	config.ProvideAllCPUMetrics = true
	config.ProvideAllMemoryMetrics = true
	config.ProvideAllNetworkMetrics = true

	receiver, err := f.CreateMetricsReceiver(context.Background(), logger, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestMonitoringAddedContainerIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))

	consumer := &exportertest.SinkMetricsExporterOld{}
	f, config := factory()
	receiver, err := f.CreateMetricsReceiver(context.Background(), logger, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	d := container.New(t)
	d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestExcludedImageProducesNoMetricsIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))

	d := container.New(t)
	d.StartImage("docker.io/library/redis:6.0.3", container.WithPortReady(6379))

	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "*redis*")

	consumer := &exportertest.SinkMetricsExporterOld{}
	receiver, err := f.CreateMetricsReceiver(context.Background(), logger, config, consumer)
	r := receiver.(*Receiver)

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Never(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "received undesired metrics")

	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestRemovedContainerRemovesRecordsIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))

	consumer := &exportertest.SinkMetricsExporterOld{}
	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "!*nginx*")
	receiver, err := f.CreateMetricsReceiver(context.Background(), logger, config, consumer)
	r := receiver.(*Receiver)

	d := container.New(t)
	nginx := d.StartImage("docker.io/library/nginx:1.17", container.WithPortReady(80))

	require.NoError(t, err, "failed creating metrics Receiver")
	require.NoError(t, r.Start(context.Background(), &testHost{
		t: t,
	}))

	desiredAmount := func(num int) func() bool {
		return func() bool {
			// We need the locks to prevent data race warnings for test activity
			r.client.inspectedContainers.lock.Lock()
			defer r.client.inspectedContainers.lock.Unlock()
			r.client.containersLock.Lock()
			defer r.client.containersLock.Unlock()
			return len(r.client.containers) == num && len(r.client.inspectedContainers.containers) == num
		}
	}

	assert.Eventuallyf(t, desiredAmount(1), 5*time.Second, 1*time.Millisecond, "failed to load container stores")
	containers := r.client.Containers()
	d.RemoveContainer(nginx)
	assert.Eventuallyf(t, desiredAmount(0), 5*time.Second, 1*time.Millisecond, "failed to clear container stores")

	// Confirm missing container paths
	results := make(chan Result, len(containers))
	r.client.FetchContainerStatsAndConvertToMetrics(context.Background(), containers[0], results)
	result := <-results
	assert.Nil(t, result.md)
	require.Error(t, result.err)

	assert.False(t, r.client.updateInspectedContainers(context.Background(), containers[0].ID))

	assert.NoError(t, r.Shutdown(context.Background()))
}
