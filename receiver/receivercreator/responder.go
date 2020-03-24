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

	"github.com/jwangsadinata/go-multimap/slicemultimap"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type responder struct {
	runner             receiverRunner
	logger             *zap.Logger
	subreceiverConfigs map[string]*subreceiverConfig
	started            *slicemultimap.MultiMap
}

var _ observer.ObserverNotify = (*responder)(nil)

// configFromEndpoint returns the configuration map for a receiver based on
// the provided endpoint.
func configFromEndpoint(endpoint observer.Endpoint) map[string]interface{} {
	var receiverEndpoint string
	switch endpoint := endpoint.(type) {
	case *observer.HostEndpoint:
		receiverEndpoint = endpoint.Target
	case *observer.PortEndpoint:
		receiverEndpoint = fmt.Sprintf("%s:%d", endpoint.Target, endpoint.Port)
	}

	return map[string]interface{}{
		"endpoint": receiverEndpoint,
	}
}

// OnAdd is called when a new endpoint appears.
func (r *responder) OnAdd(endpoints []observer.Endpoint) {
	for _, e := range endpoints {
		if r.started.ContainsKey(e.ID()) {
			r.logger.DPanic("endpoint unexpectedly already present", zap.Stringer("endpoint", e))
			continue
		}

		for receiverName, cfg := range r.subreceiverConfigs {
			matches, err := ruleMatches(e, cfg.Rule)
			if err != nil {
				r.logger.Error("failed to apply rule to endpoint", zap.Stringer("endpoint", e), zap.Error(err))
				continue
			}

			if !matches {
				continue
			}

			envCfg := configFromEndpoint(e)
			rcvr, err := r.runner.StartReceiver(cfg, envCfg)
			if err != nil {
				// TODO: need to figure out how not to log sensitive things (credentials, secrets, etc.)
				r.logger.Error("failed to run receiver", zap.Stringer("endpoint", e),
					zap.Error(err), zap.Reflect("cfg", cfg.config), zap.Reflect("envCfg", envCfg))
				continue
			}

			r.logger.Info("started receiver", zap.String("name", receiverName), zap.Stringer("endpoint", e))
			r.started.Put(e.ID(), rcvr)
		}
	}
}

// OnRemove is called when endpoints are gone.
func (r *responder) OnRemove(endpoints []observer.Endpoint) {
	for _, e := range endpoints {
		rcvrs, _ := r.started.Get(e.ID())
		for _, rcvr := range rcvrs {
			if err := r.runner.StopReceiver(rcvr.(component.Receiver)); err != nil {
				r.logger.Error("failed stopping receiver", zap.Any("endpoint", e))
			}
		}
		r.started.RemoveAll(e.ID())
	}
}

// OnChange is called when an endpoint is still present but has been modified.
func (r *responder) OnChange(endpoints []observer.Endpoint) {
	// TODO: Make this only restart receivers if something relevant has actually changed. (ie the configuration)
	r.OnRemove(endpoints)
	r.OnAdd(endpoints)
}
