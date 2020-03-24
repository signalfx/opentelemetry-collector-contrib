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
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type receiverRunner interface {
	StartReceiver(cfg *subreceiverConfig, subConfigFromEnv map[string]interface{}) (component.Receiver, error)
	StopReceiver(rcvr component.Receiver) error
	Shutdown() error
}

// runner manages loading and running receivers that have been started at runtime.
type runner struct {
	sync.Mutex
	logger             *zap.Logger
	host               component.Host
	consumer           consumer.MetricsConsumerOld
	parentReceiverName string
	receivers          map[component.Receiver]struct{}
}

var _ receiverRunner = (*runner)(nil)

// Shutdown all started receivers.
func (lo *runner) Shutdown() error {
	lo.Lock()
	defer lo.Unlock()

	var errs []error

	for recvr := range lo.receivers {
		if err := recvr.Shutdown(); err != nil {
			// TODO: Should keep track of which receiver the error is associated with
			// but require some restructuring.
			errs = append(errs, err)
		}
	}

	lo.receivers = nil

	if len(errs) > 0 {
		// Or maybe just return a general error and log the failed shutdowns?
		return fmt.Errorf("shutdown on %d receivers failed: %v", len(errs), oterr.CombineErrors(errs))
	}

	return nil
}

// loadRuntimeReceiverConfig loads the given staticSubConfig merged with config values
// that may have been discovered at runtime.
func (lo *runner) loadRuntimeReceiverConfig(
	staticSubConfig *subreceiverConfig,
	subConfigFromEnv map[string]interface{}) (configmodels.Receiver, error) {
	// Load config under <receiver>/<id> since loadReceiver and CustomUnmarshaler expects this structure.
	viperConfig := viper.New()
	viperConfig.Set(staticSubConfig.fullName, map[string]interface{}{})
	subreceiverConfig := viperConfig.Sub(staticSubConfig.fullName)

	// Merge in the config values specified in the config file.
	if err := subreceiverConfig.MergeConfigMap(staticSubConfig.config); err != nil {
		return nil, fmt.Errorf("failed to merge subreceiver config from config file: %v", err)
	}

	// Merge in subConfigFromEnv containing values discovered at runtime.
	if err := subreceiverConfig.MergeConfigMap(subConfigFromEnv); err != nil {
		return nil, fmt.Errorf("failed to merge subreceiver config from discovered runtime values: %v", err)
	}

	receiverConfig, err := config.LoadReceiver(staticSubConfig.fullName, viperConfig, factories)
	if err != nil {
		return nil, fmt.Errorf("failed to load subreceiver config: %v", err)
	}
	// Sets dynamically created receiver to something like receiver_creator/1/redis{endpoint="localhost:6380"}.
	// TODO: Need to make sure this is unique (just endpoint is probably not totally sufficient).
	receiverConfig.SetName(fmt.Sprintf("%s/%s{endpoint=%q}", lo.parentReceiverName, staticSubConfig.fullName, subreceiverConfig.GetString("endpoint")))
	return receiverConfig, nil
}

// createRuntimeReceiver creates a receiver that is to discovered at runtime
func (lo *runner) createRuntimeReceiver(cfg configmodels.Receiver) (component.MetricsReceiver, error) {
	factory, err := factories.get(cfg.Type())
	if err != nil {
		return nil, err
	}
	receiverFactory := factory.(component.ReceiverFactoryOld)
	return receiverFactory.CreateMetricsReceiver(lo.logger, cfg, lo.consumer)
}

// StartReceiver starts the given staticSubConfig merged with config values
// that may have been discovered at runtime.
func (lo *runner) StartReceiver(staticSubConfig *subreceiverConfig, subConfigFromEnv map[string]interface{}) (component.Receiver, error) {
	lo.Lock()
	defer lo.Unlock()

	runtimeCfg, err := lo.loadRuntimeReceiverConfig(staticSubConfig, subConfigFromEnv)
	if err != nil {
		lo.logger.Error("failed loading runtime receiver", zap.String("receiverType", staticSubConfig.receiverType), zap.Error(err))
	}
	rcvr, err := lo.createRuntimeReceiver(runtimeCfg)
	if err != nil {
		lo.logger.Error("failed creating runtime receiver", zap.String("receiverType", staticSubConfig.receiverType), zap.Error(err))
	}
	if err := rcvr.Start(lo.host); err != nil {
		return nil, err
	}

	lo.receivers[rcvr] = struct{}{}
	return rcvr, nil
}

// StopReceiver stops the given receiver.
func (lo *runner) StopReceiver(rcvr component.Receiver) error {
	lo.Lock()
	defer lo.Unlock()
	delete(lo.receivers, rcvr)
	return rcvr.Shutdown()
}
