/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package strategy

import (
	"fmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
)

type Strategy interface {
	// Add new objects
	Add(obj interface{}) error

	// Update objects
	Update(obj interface{}) error

	// Delete objects
	Delete(obj interface{}) error
}

type predictiveStrategy struct {
	resourceStore store.ResourceStore

	// for each node keep its NodeInfo
	nodeInfo map[string]*schedulercache.NodeInfo
}

func (s *predictiveStrategy) addPod(pod *v1.Pod) error {
	// No need to update any node.
	// The scheduler keep resources consumed by all pods in its scheduler cache
	// which is than confronted with pod's node Allocatable field.

	// mark the pod as running rather than keeping the phase empty
	pod.Status.Phase = v1.PodRunning

	// here asuming the pod is already in the resource storage
	// so the update is needed to emit update event in case a handler is registered
	err := s.resourceStore.Update("pods", metav1.Object(pod))
	if err != nil {
		return fmt.Errorf("Unable to add new node: %v", err)
	}

	return nil
}

// Simulate creation of new object (only pods currently supported)
// The method returns error on the first occurence of processing error.
// If so, all succesfully processed objects up to the first failed are reflected in the resource store.
func (s *predictiveStrategy) Add(obj interface{}) error {
	switch item := obj.(type) {
	case *v1.Pod:
		return s.addPod(item)
	default:
		return fmt.Errorf("resource kind not recognized")
	}
}

func (s *predictiveStrategy) Update(obj interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func (s *predictiveStrategy) Delete(obj interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func NewPredictiveStrategy(resourceStore store.ResourceStore) *predictiveStrategy {
	return &predictiveStrategy{
		resourceStore: resourceStore,
	}
}
