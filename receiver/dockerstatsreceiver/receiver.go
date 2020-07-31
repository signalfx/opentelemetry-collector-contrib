// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerstatsreceiver

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ component.MetricsReceiver = (*Receiver)(nil)

type Receiver struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.MetricsConsumerOld
	client       *DockerClient
	runner       *interval.Runner
	obsCtx       context.Context
	runnerCancel context.CancelFunc
	transport    string
}

func NewReceiver(
	_ context.Context,
	config *Config,
	logger *zap.Logger,
	nextConsumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("could not determine receiver transport: %w", err)
	}

	receiver := Receiver{
		config:       config,
		nextConsumer: nextConsumer,
		logger:       logger,
		transport:    parsed.Scheme,
	}

	return &receiver, nil
}

func (r *Receiver) Start(ctx context.Context, host component.Host) error {
	var err error
	r.client, err = NewDockerClient(r.config, r.logger)
	if err != nil {
		return err
	}

	r.obsCtx = obsreport.ReceiverContext(ctx, typeStr, r.transport, r.config.Name())

	runnableCtx, cancel := context.WithCancel(ctx)

	runnable := newRunnable(runnableCtx, cancel, r)
	r.runner = interval.NewRunner(r.config.CollectionInterval, runnable)
	r.runnerCancel = cancel

	go func() {
		if err := r.runner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (r *Receiver) Shutdown(ctx context.Context) error {
	r.runnerCancel()
	r.runner.Stop()
	return nil
}

type Runnable struct {
	ctx               context.Context
	cancel            func()
	receiver          *Receiver
	successfullySetup bool
}

var _ interval.Runnable = (*Runnable)(nil)

func newRunnable(
	ctx context.Context,
	cancel func(),
	receiver *Receiver,
) *Runnable {
	return &Runnable{
		ctx:      ctx,
		cancel:   cancel,
		receiver: receiver,
	}
}

func (r *Runnable) Setup() error {
	err := r.receiver.client.StartWatchingContainers(r.ctx)
	if err == nil {
		r.successfullySetup = true
	}
	return err
}

type Result struct {
	md  *consumerdata.MetricsData
	err error
}

func (r *Runnable) Run() error {
	if !r.successfullySetup {
		return r.Setup()
	}

	c := obsreport.StartMetricsReceiveOp(r.receiver.obsCtx, typeStr, r.receiver.transport)

	containers := r.receiver.client.Containers()
	results := make(chan Result, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, container := range containers {
		go func(dc DockerContainer) {
			r.receiver.client.FetchContainerStatsAndConvertToMetrics(r.ctx, dc, results)
			wg.Done()
		}(container)
	}

	wg.Wait()
	close(results)

	numPoints := 0
	numTimeSeries := 0
	var lastErr error
	for result := range results {
		var err error
		if result.md != nil {
			var nts, np int
			if result.md != nil {
				nts, np = obsreport.CountMetricPoints(*result.md)
			}
			numTimeSeries += nts
			numPoints += np

			err = r.receiver.nextConsumer.ConsumeMetricsData(r.ctx, *result.md)
		} else {
			err = result.err
		}

		if err != nil {
			lastErr = err
		}
	}

	obsreport.EndMetricsReceiveOp(c, typeStr, numPoints, numTimeSeries, lastErr)
	return nil
}
