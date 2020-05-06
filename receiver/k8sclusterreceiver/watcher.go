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

package k8sclusterreceiver

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/autoscaling/v2beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

type resourceWatcher struct {
	client                kubernetes.Interface
	sharedInformerFactory informers.SharedInformerFactory
	dataCollector         *collection.DataCollector
	logger                *zap.Logger
	metadataConsumers     []metadataConsumer
}

type metadataConsumer func(metadata map[string]*collection.KubernetesMetadataUpdate) error

// newResourceWatcher creates a Kubernetes resource watcher.
func newResourceWatcher(logger *zap.Logger, config *Config,
	client kubernetes.Interface) (*resourceWatcher, error) {
	rw := &resourceWatcher{
		client:        client,
		logger:        logger,
		dataCollector: collection.NewDataCollector(logger, config.NodeConditionTypesToReport),
	}

	rw.prepareSharedInformerFactory()

	return rw, nil
}

func (rw *resourceWatcher) prepareSharedInformerFactory() {
	factory := informers.NewSharedInformerFactoryWithOptions(rw.client, 0)

	// Add shared informers for each resource type that has to be watched.
	rw.setupInformers(&corev1.Pod{}, factory.Core().V1().Pods().Informer())
	rw.setupInformers(&corev1.Node{}, factory.Core().V1().Nodes().Informer())
	rw.setupInformers(&corev1.Namespace{}, factory.Core().V1().Namespaces().Informer())
	rw.setupInformers(&corev1.ReplicationController{},
		factory.Core().V1().ReplicationControllers().Informer(),
	)
	rw.setupInformers(&corev1.ResourceQuota{}, factory.Core().V1().ResourceQuotas().Informer())
	rw.setupInformers(&corev1.Service{}, factory.Core().V1().Services().Informer())
	rw.setupInformers(&appsv1.DaemonSet{}, factory.Apps().V1().DaemonSets().Informer())
	rw.setupInformers(&appsv1.Deployment{}, factory.Apps().V1().Deployments().Informer())
	rw.setupInformers(&appsv1.ReplicaSet{}, factory.Apps().V1().ReplicaSets().Informer())
	rw.setupInformers(&appsv1.StatefulSet{}, factory.Apps().V1().StatefulSets().Informer())
	rw.setupInformers(&batchv1.Job{}, factory.Batch().V1().Jobs().Informer())
	rw.setupInformers(&batchv1beta1.CronJob{}, factory.Batch().V1beta1().CronJobs().Informer())
	rw.setupInformers(&v2beta1.HorizontalPodAutoscaler{},
		factory.Autoscaling().V2beta1().HorizontalPodAutoscalers().Informer(),
	)

	rw.sharedInformerFactory = factory
}

// startWatchingResources starts up all informers.
func (rw *resourceWatcher) startWatchingResources(stopper <-chan struct{}) {
	rw.sharedInformerFactory.Start(stopper)
}

// setupInformers adds event handlers to informers and setups a metadataStore.
func (rw *resourceWatcher) setupInformers(o runtime.Object, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rw.onAdd,
		UpdateFunc: rw.onUpdate,
		DeleteFunc: rw.onDelete,
	})
	rw.dataCollector.SetupMetadataStore(o, informer.GetStore())
}

func (rw *resourceWatcher) onAdd(obj interface{}) {
	rw.dataCollector.SyncMetrics(obj)

	// Sync metadata only if there's at least one destination for it to sent.
	if len(rw.metadataConsumers) == 0 {
		return
	}

	newMetadata := rw.dataCollector.SyncMetadata(obj)
	kubernetesMetadataUpdate := collection.GetKubernetesMetadataUpdate(
		map[string]*collection.KubernetesMetadata{}, newMetadata,
	)

	if len(kubernetesMetadataUpdate) == 0 {
		return
	}

	rw.dispatchMetadataUpdate(kubernetesMetadataUpdate)
}

func (rw *resourceWatcher) onDelete(obj interface{}) {
	rw.dataCollector.RemoveFromMetricsStore(obj)
}

func (rw *resourceWatcher) onUpdate(oldObj, newObj interface{}) {
	// Sync metrics from the new object
	rw.dataCollector.SyncMetrics(newObj)

	// Sync metadata only if there's at least one destination for it to sent.
	if len(rw.metadataConsumers) == 0 {
		return
	}

	oldMetadata := rw.dataCollector.SyncMetadata(oldObj)
	newMetadata := rw.dataCollector.SyncMetadata(newObj)

	kubernetesMetadataUpdate := collection.GetKubernetesMetadataUpdate(oldMetadata, newMetadata)

	if len(kubernetesMetadataUpdate) == 0 {
		return
	}

	rw.dispatchMetadataUpdate(kubernetesMetadataUpdate)
}

func (rw *resourceWatcher) setupMetadataExporters(
	exporters map[configmodels.DataType]map[configmodels.Exporter]component.Exporter,
	metadataExportersFromConfig []string) error {
	var metadataExporters []metadataConsumer
	metadataExportersSet := utils.StringSliceToMap(metadataExportersFromConfig)

	for pipelineType, exps := range exporters {
		for cfg, exp := range exps {
			if !metadataExportersSet[string(cfg.Type())] {
				continue
			}
			kme, ok := exp.(collection.KubernetesMetadataExporter)
			if !ok {
				return fmt.Errorf("%v exporter does not implement KubernetesMetadataExporter", cfg.Type())
			}
			metadataExporters = append(metadataExporters, kme.ConsumeKubernetesMetadata)
			rw.logger.Info("Configured Kubernetes MetadataExporter",
				zap.String("pipeline_type", pipelineType.GetString()),
				zap.String("exporter_type", string(cfg.Type())),
			)
		}
	}

	rw.metadataConsumers = metadataExporters
	return nil
}

func (rw *resourceWatcher) dispatchMetadataUpdate(km map[string]*collection.KubernetesMetadataUpdate) {
	for _, consume := range rw.metadataConsumers {
		consume(km)
	}
}
