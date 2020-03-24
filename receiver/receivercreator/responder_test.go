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
	"reflect"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type mockLoader struct {
	mock.Mock
}

type mockReceiver struct {
}

func (m *mockReceiver) Start(host component.Host) error {
	return nil
}

func (m *mockReceiver) Shutdown() error {
	return nil
}

func (m *mockLoader) StartReceiver(cfg *subreceiverConfig, subConfigFromEnv map[string]interface{}) (component.Receiver, error) {
	args := m.Called(cfg, subConfigFromEnv)
	return args.Get(0).(component.Receiver), args.Error(1)
}

func (m *mockLoader) StopReceiver(rcvr component.Receiver) error {
	args := m.Called(rcvr)
	return args.Error(0)
}

func (m *mockLoader) Shutdown() error {
	return nil
}

func Test_configFromEndpoint(t *testing.T) {
	type args struct {
		endpoint observer.Endpoint
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{"from pod endpoint", args{observer.NewPortEndpoint("pod1", "1.2.3.4", 80, nil)}, map[string]interface{}{
			"endpoint": "1.2.3.4:80",
		}},
		{"from host endpoint", args{observer.NewHostEndpoint("pod1", "1.2.3.4", nil)}, map[string]interface{}{
			"endpoint": "1.2.3.4",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configFromEndpoint(tt.args.endpoint); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("configFromEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOnAdd(t *testing.T) {
	rc := exampleReceiverCreator(t, "receiver_creator/1")
	loader := &mockLoader{}
	rcvr := &mockReceiver{}
	loader.On("StartReceiver", rc.cfg.subreceiverConfigs["examplereceiver/2"], map[string]interface{}{
		"endpoint": "1.2.3.4",
	}).Return(rcvr, nil)

	responder := rc.newResponder()
	responder.runner = loader
	endpoint := observer.NewHostEndpoint("host1", "1.2.3.4", nil)
	responder.OnAdd([]observer.Endpoint{endpoint})

	loader.AssertExpectations(t)

	assert.Equal(t, 1, responder.started.Size())
	assert.True(t, responder.started.Contains(endpoint.ID(), rcvr))
}

func TestOnRemove(t *testing.T) {
	rc := exampleReceiverCreator(t, "receiver_creator/1")
	loader := &mockLoader{}
	rcvr := &mockReceiver{}
	loader.On("StartReceiver", rc.cfg.subreceiverConfigs["examplereceiver/2"], map[string]interface{}{
		"endpoint": "1.2.3.4",
	}).Return(rcvr, nil)
	loader.On("StopReceiver", rcvr).Return(nil)

	responder := rc.newResponder()
	responder.runner = loader
	endpoint := observer.NewHostEndpoint("host1", "1.2.3.4", nil)

	responder.OnAdd([]observer.Endpoint{endpoint})
	require.Equal(t, 1, responder.started.Size())

	responder.OnRemove([]observer.Endpoint{endpoint})

	loader.AssertExpectations(t)
	assert.Equal(t, 0, responder.started.Size())
}

func TestOnChange(t *testing.T) {
	rc := exampleReceiverCreator(t, "receiver_creator/1")
	loader := &mockLoader{}
	rcvr := &mockReceiver{}
	loader.On("StartReceiver", rc.cfg.subreceiverConfigs["examplereceiver/2"], map[string]interface{}{
		"endpoint": "1.2.3.4",
	}).Return(rcvr, nil)
	loader.On("StopReceiver", rcvr).Return(nil)
	loader.On("StartReceiver", rc.cfg.subreceiverConfigs["examplereceiver/2"], map[string]interface{}{
		"endpoint": "1.2.3.4",
	}).Return(rcvr, nil)

	responder := rc.newResponder()
	responder.runner = loader
	endpoint := observer.NewHostEndpoint("host1", "1.2.3.4", nil)

	responder.OnAdd([]observer.Endpoint{endpoint})
	require.Equal(t, 1, responder.started.Size())

	responder.OnChange([]observer.Endpoint{endpoint})

	loader.AssertExpectations(t)
	assert.Equal(t, 1, responder.started.Size())
}
