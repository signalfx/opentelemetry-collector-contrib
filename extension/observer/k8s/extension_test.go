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
	"sync"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	framework "k8s.io/client-go/tools/cache/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// NewPod is a helper function for creating Pods for testing.
func NewPod(name, host string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1.PodSpec{
			NodeName: host,
		},
		Status: v1.PodStatus{
			Conditions: []v1.PodCondition{
				{
					Type:   v1.PodReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	return pod
}

func TestNewExtension(t *testing.T) {
	listWatch := framework.NewFakeControllerSource()
	factory := &Factory{}
	ext, err := New(zap.NewNop(), factory.CreateDefaultConfig().(*Config), listWatch)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

type endpointSink struct {
	sync.Mutex
	added   []observer.Endpoint
	removed []observer.Endpoint
	changed []observer.Endpoint
}

func (e *endpointSink) OnAdd(added []observer.Endpoint) {
	e.Lock()
	defer e.Unlock()
	e.added = append(e.added, added...)
}

func (e *endpointSink) OnRemove(removed []observer.Endpoint) {
	e.Lock()
	defer e.Unlock()
	e.removed = append(e.removed, removed...)
}

func (e *endpointSink) OnChange(changed []observer.Endpoint) {
	e.Lock()
	defer e.Unlock()
	e.changed = append(e.removed, changed...)
}

var _ observer.ObserverNotify = (*endpointSink)(nil)

func TestExtensionObserve(t *testing.T) {
	listWatch := framework.NewFakeControllerSource()
	factory := &Factory{}
	ext, err := New(zap.NewNop(), factory.CreateDefaultConfig().(*Config), listWatch)
	require.NoError(t, err)
	require.NotNil(t, ext)
	obs := ext.(*k8sObserver)

	listWatch.Add(NewPod("pod1", "localhost"))

	require.NoError(t, ext.Start(component.NewMockHost()))

	sink := &endpointSink{}
	obs.ListAndWatch(sink)

	assert.Eventually(t, func() bool {
		sink.Lock()
		defer sink.Unlock()
		return len(sink.added) == 1
	}, 1*time.Second, 100*time.Millisecond)

	assert.Equal(t, observer.NewPortEndpoint("pod1", "localhost", 80, nil), sink.added[0])

	defer require.NoError(t, ext.Shutdown())
}
