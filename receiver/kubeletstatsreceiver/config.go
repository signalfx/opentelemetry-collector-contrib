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

package kubeletstatsreceiver

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

var _ configmodels.Receiver = (*Config)(nil)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	kubelet.ClientConfig          `mapstructure:",squash"`
	CollectionInterval            time.Duration `mapstructure:"collection_interval"`
}
