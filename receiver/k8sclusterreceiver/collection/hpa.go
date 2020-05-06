// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/translator/conventions"
	"k8s.io/api/autoscaling/v2beta1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

var hpaMaxReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/hpa/max_replicas",
	Description: "Maximum number of replicas to which the autoscaler can scale up",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var hpaMinReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/hpa/min_replicas",
	Description: "Minimum number of replicas to which the autoscaler can scale down",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var hpaCurrentReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/hpa/current_replicas",
	Description: "Current number of pod replicas managed by this autoscaler",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

var hpaDesiredReplicasMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/hpa/desired_replicas",
	Description: "Desired number of pod replicas managed by this autoscaler",
	Unit:        "1",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func getMetricsForHPA(hpa *v2beta1.HorizontalPodAutoscaler) []*resourceMetrics {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: hpaMaxReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Spec.MaxReplicas)),
			},
		},
		{
			MetricDescriptor: hpaMinReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(*hpa.Spec.MinReplicas)),
			},
		},
		{
			MetricDescriptor: hpaCurrentReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Status.CurrentReplicas)),
			},
		},
		{
			MetricDescriptor: hpaDesiredReplicasMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(hpa.Status.DesiredReplicas)),
			},
		},
	}

	return []*resourceMetrics{
		{
			resource: getResourceForHPA(hpa),
			metrics:  metrics,
		},
	}
}

func getResourceForHPA(hpa *v2beta1.HorizontalPodAutoscaler) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			k8sKeyHPAUID:                      string(hpa.UID),
			k8sKeyHPAName:                     hpa.Name,
			conventions.AttributeK8sNamespace: hpa.Namespace,
			conventions.AttributeK8sCluster:   hpa.ClusterName,
		},
	}
}

func getMetadataForHPA(hpa *v2beta1.HorizontalPodAutoscaler) map[string]*KubernetesMetadata {
	return map[string]*KubernetesMetadata{
		string(hpa.UID): getGenericMetadata(&hpa.ObjectMeta, "hpa"),
	}
}
