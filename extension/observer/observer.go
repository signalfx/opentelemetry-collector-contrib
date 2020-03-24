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

package observer

import (
	"fmt"
)

// Protocol defines network protocol for container ports.
type Protocol string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Protocol = "UDP"
)

type Endpoint interface {
	ID() string
	String() string
}

type endpointBase struct {
	id     string
	Target string
	Labels map[string]string
}

func (e *endpointBase) ID() string {
	return e.id
}

type HostEndpoint struct {
	endpointBase
}

func (h *HostEndpoint) String() string {
	return fmt.Sprintf("HostEndpoint{id: %v, Target: %v, Labels: %v}", h.ID(), h.Target, h.Labels)
}

func NewHostEndpoint(id string, target string, labels map[string]string) *HostEndpoint {
	return &HostEndpoint{endpointBase{
		id:     id,
		Target: target,
		Labels: labels,
	}}
}

var _ Endpoint = (*HostEndpoint)(nil)

type PortEndpoint struct {
	endpointBase
	Port int32
}

func (p *PortEndpoint) String() string {
	return fmt.Sprintf("PortEndpoint{ID: %v, Target: %v, Port: %d, Labels: %v}", p.ID(), p.Target, p.Port, p.Labels)
}

func NewPortEndpoint(id string, target string, port int32, labels map[string]string) *PortEndpoint {
	return &PortEndpoint{endpointBase: endpointBase{
		id:     id,
		Target: target,
		Labels: labels,
	}, Port: port}
}

var _ Endpoint = (*PortEndpoint)(nil)

type Observer interface {
	ListAndWatch(notify ObserverNotify)
}

type ObserverNotify interface {
	OnAdd([]Endpoint)
	OnRemove([]Endpoint)
	OnChange([]Endpoint)
}
