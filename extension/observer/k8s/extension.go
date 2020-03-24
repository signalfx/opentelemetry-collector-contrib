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

package k8s

import (
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/uber-go/atomic"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type k8sObserver struct {
	logger   *zap.Logger
	informer cache.SharedInformer
	running  atomic.Bool
	stop     chan struct{}
}

func (k *k8sObserver) Start(host component.Host) error {
	go k.informer.Run(k.stop)
	return nil
}

func (k *k8sObserver) Shutdown() error {
	close(k.stop)
	return nil
}

var _ (component.ServiceExtension) = (*k8sObserver)(nil)

func (k *k8sObserver) ListAndWatch(notify observer.ObserverNotify) {
	k.informer.AddEventHandler(&responder{notify: notify})
}

// New creates a new k8s observer extension.
func New(logger *zap.Logger, config *Config, listWatch cache.ListerWatcher) (component.ServiceExtension, error) {
	informer := cache.NewSharedInformer(listWatch, &v1.Pod{}, 0)
	return &k8sObserver{logger: logger, informer: informer, stop: make(chan struct{})}, nil
}
