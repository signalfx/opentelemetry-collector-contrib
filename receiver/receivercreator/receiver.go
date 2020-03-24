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
	"errors"
	"fmt"

	"github.com/jwangsadinata/go-multimap/slicemultimap"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
	factories          = buildFactoryMap(&config.ExampleReceiverFactory{})
)

type factoryMap map[string]component.ReceiverFactoryBase

func (fm factoryMap) get(receiverType string) (component.ReceiverFactoryBase, error) {
	if factory, ok := factories[receiverType]; ok {
		return factory, nil
	}
	return nil, fmt.Errorf("factory does not exist for receiver type %q", receiverType)
}

func buildFactoryMap(factories ...component.ReceiverFactoryBase) factoryMap {
	ret := map[string]component.ReceiverFactoryBase{}
	for _, f := range factories {
		ret[f.Type()] = f
	}
	return ret
}

var _ component.MetricsReceiver = (*receiverCreator)(nil)

// receiverCreator implements consumer.MetricsConsumer.
type receiverCreator struct {
	nextConsumer consumer.MetricsConsumerOld
	logger       *zap.Logger
	cfg          *Config
	observer     observer.Observer
	loader       *runner
}

// New creates the receiver_creator with the given parameters.
func New(logger *zap.Logger, nextConsumer consumer.MetricsConsumerOld, cfg *Config) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextConsumer
	}

	r := &receiverCreator{
		logger:       logger,
		nextConsumer: nextConsumer,
		cfg:          cfg,
	}
	return r, nil
}

// Start receiver_creator.
func (dr *receiverCreator) Start(host component.Host) error {
	// TODO: Lookup observer at runtime once available.
	// TODO: Cannot currently cancel observer watch.

	// Loader controls starting and stopping receivers.
	dr.loader = dr.newLoader(host)

	// Responder triggers start/stopping of receivers based on events received from observer.
	responder := dr.newResponder()

	dr.observer.ListAndWatch(responder)
	return nil
}

func (dr *receiverCreator) newResponder() *responder {
	return &responder{runner: dr.loader, logger: dr.logger, subreceiverConfigs: dr.cfg.subreceiverConfigs,
		started: slicemultimap.New()}
}

func (dr *receiverCreator) newLoader(host component.Host) *runner {
	return &runner{parentReceiverName: dr.cfg.Name(), host: host, consumer: dr.nextConsumer,
		receivers: map[component.Receiver]struct{}{}}
}

// Shutdown stops the receiver_creator and all its receivers started at runtime.
func (dr *receiverCreator) Shutdown() error {
	// TODO: maybe if can't cancel the watch make the responder not forward
	// events to runner?
	return dr.loader.Shutdown()
}
