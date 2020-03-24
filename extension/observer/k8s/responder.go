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
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// TODO: test object separately
type responder struct {
	notify observer.ObserverNotify
}

func (r *responder) OnAdd(obj interface{}) {
	pod := obj.(*v1.Pod)
	r.notify.OnAdd(convertPodToEndpoints(pod))
}

func convertPodToEndpoints(pod *v1.Pod) []observer.Endpoint {
	// TODO
	return []observer.Endpoint{observer.NewPortEndpoint("localhost", "localhost", 80, nil)}
}

func (r *responder) OnUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*v1.Pod)
	newPod := newObj.(*v1.Pod)

	oldEndpoints := convertPodToEndpoints(oldPod)
	newEndpoints := convertPodToEndpoints(newPod)
	_, _ = oldEndpoints, newEndpoints

	// Do diff based on id.
	//_ = oldEndpoints
	//_ = newEndpoints
	// OnDelete endpoints that are no longer present in newEndpoints.
	// OnAdd endpoints that are only in newEndpoints.
	// OnChange endpoints (by id) that differ between the old and new.

	// TODO: can changes be missed where a pod is deleted but we don't
	// send remove notifications for some of its endpoints? If not provable
	// then maybe keep track of pod -> endpoint association to be sure
	// they are all cleaned up.
}

func (r *responder) OnDelete(obj interface{}) {
	var pod *v1.Pod
	switch o := obj.(type) {
	case *cache.DeletedFinalStateUnknown:
		// Assuming we never saw the pod state where new endpoints would have been created
		// to begin with it seems that we can't leak endpoints here.
		pod = o.Obj.(*v1.Pod)
	case *v1.Pod:
		pod = o
	default:
		return
	}
	r.notify.OnRemove(convertPodToEndpoints(pod))
}
