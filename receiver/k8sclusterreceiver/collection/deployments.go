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

package collection

import (
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/translator/conventions"
	appsv1 "k8s.io/api/apps/v1"
)

func getMetricsForDeployment(dep *appsv1.Deployment) []*resourceMetrics {
	if dep.Spec.Replicas == nil {
		return nil
	}

	return []*resourceMetrics{
		{
			resource: getResourceForDeployment(dep),
			metrics: getReplicaMetrics(
				"deployment",
				*dep.Spec.Replicas,
				dep.Status.AvailableReplicas,
			),
		},
	}
}

func getResourceForDeployment(dep *appsv1.Deployment) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			k8sKeyDeploymentUID:               string(dep.UID),
			k8sKeyDeploymentName:              dep.Name,
			conventions.AttributeK8sNamespace: dep.Namespace,
			conventions.AttributeK8sCluster:   dep.ClusterName,
		},
	}
}

func getMetadataForDeployment(dep *appsv1.Deployment) map[string]*KubernetesMetadata {
	rm := getGenericMetadata(&dep.ObjectMeta, k8sKindDeployment)
	rm.properties[k8sKeyDeploymentName] = dep.Name
	return map[string]*KubernetesMetadata{string(dep.UID): rm}
}
