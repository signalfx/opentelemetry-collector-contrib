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

package signalfxexporter

import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"

type dimensionUpdate struct {
	dimensionKey   string
	dimensionValue string
	properties     map[string]string
	tags           map[string]bool
}

func getDimensionUpdateFromMetadata(metadata collection.KubernetesMetadata) dimensionUpdate {
	properties, tags := getPropertiesAndTags(metadata)

	return dimensionUpdate{
		dimensionKey:   metadata.ResourceIDKey,
		dimensionValue: metadata.ResourceID,
		properties:     properties,
		tags:           tags,
	}
}

func getPropertiesAndTags(metadata collection.KubernetesMetadata) (map[string]string, map[string]bool) {
	properties, tags := map[string]string{}, map[string]bool{}

	for key, val := range metadata.Properties {
		if key == "" {
			continue
		}

		if val == "" {
			tags[key] = true
		} else {
			properties[key] = val
		}
	}

	return properties, tags
}
