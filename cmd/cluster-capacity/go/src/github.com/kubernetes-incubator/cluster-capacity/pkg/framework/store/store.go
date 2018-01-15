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

package store

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
)

type ResourceStore interface {
	Add(resource ccapi.ResourceType, obj interface{}) error
	Update(resource ccapi.ResourceType, obj interface{}) error
	Delete(resource ccapi.ResourceType, obj interface{}) error
	List(resource ccapi.ResourceType) []interface{}
	Get(resource ccapi.ResourceType, obj interface{}) (item interface{}, exists bool, err error)
	GetByKey(resource ccapi.ResourceType, key string) (item interface{}, exists bool, err error)
	RegisterEventHandler(resource ccapi.ResourceType, handler cache.ResourceEventHandler) error
	// Replace will delete the contents of the store, using instead the
	// given list. Store takes ownership of the list, you should not reference
	// it after calling this function.
	Replace(resource ccapi.ResourceType, items []interface{}, resourceVersion string) error

	Resources() []ccapi.ResourceType
}

type resourceStore struct {
	// Caches modifed by emulation strategy
	PodCache     cache.Store
	NodeCache    cache.Store
	PVCCache     cache.Store
	PVCache      cache.Store
	ServiceCache cache.Store

	resourceToCache map[ccapi.ResourceType]cache.Store
	eventHandler    map[ccapi.ResourceType]cache.ResourceEventHandler
}

// Add resource obj to store and emit event handler if set
func (s *resourceStore) Add(resource ccapi.ResourceType, obj interface{}) error {
	cacheL, exists := s.resourceToCache[resource]
	if !exists {
		return fmt.Errorf("Resource %s not recognized", resource)
	}

	err := cacheL.Add(obj)
	if err != nil {
		return fmt.Errorf("Unable to add %s: %v", resource, err)
	}

	handler, found := s.eventHandler[resource]
	if !found {
		return nil
	}
	handler.OnAdd(obj)
	return nil
}

// Update resource obj to store and emit event handler if set
func (s *resourceStore) Update(resource ccapi.ResourceType, obj interface{}) error {
	cacheL, exists := s.resourceToCache[resource]
	if !exists {
		return fmt.Errorf("Resource %s not recognized", resource)
	}

	err := cacheL.Update(obj)
	if err != nil {
		return fmt.Errorf("Unable to update %s: %v", resource, err)
	}

	handler, found := s.eventHandler[resource]
	if !found {
		return nil
	}
	handler.OnUpdate(struct{}{}, obj)
	return nil
}

// Delete resource obj to store and emit event handler if set
func (s *resourceStore) Delete(resource ccapi.ResourceType, obj interface{}) error {
	cacheL, exists := s.resourceToCache[resource]
	if !exists {
		return fmt.Errorf("Resource %s not recognized", resource)
	}

	err := cacheL.Delete(obj)
	if err != nil {
		return fmt.Errorf("Unable to delete %s: %v", resource, err)
	}

	handler, found := s.eventHandler[resource]
	if !found {
		return nil
	}
	handler.OnDelete(obj)
	return nil
}

func (s *resourceStore) List(resource ccapi.ResourceType) []interface{} {
	if cacheL, exists := s.resourceToCache[resource]; exists {
		return cacheL.List()
	}
	return nil
}

func (s *resourceStore) Get(resource ccapi.ResourceType, obj interface{}) (item interface{}, exists bool, err error) {
	cacheL, exists := s.resourceToCache[resource]
	if !exists {
		return nil, false, fmt.Errorf("Resource %s not recognized", resource)
	}

	return cacheL.Get(obj)
}

func (s *resourceStore) GetByKey(resource ccapi.ResourceType, key string) (item interface{}, exists bool, err error) {
	cacheL, exists := s.resourceToCache[resource]
	if !exists {
		return nil, false, fmt.Errorf("Resource %s not recognized", resource)
	}
	return cacheL.GetByKey(key)
}

func (s *resourceStore) RegisterEventHandler(resource ccapi.ResourceType, handler cache.ResourceEventHandler) error {
	s.eventHandler[resource] = handler
	return nil
}

// Replace will delete the contents of the store, using instead the
// given list. Store takes ownership of the list, you should not reference
// it after calling this function.
func (s *resourceStore) Replace(resource ccapi.ResourceType, items []interface{}, resourceVersion string) error {
	if cacheL, exists := s.resourceToCache[resource]; exists {
		err := cacheL.Replace(items, resourceVersion)
		if err != nil {
			return err
		}
		// send one add event for each item
		handler, found := s.eventHandler[resource]
		if !found {
			return nil
		}
		for _, obj := range items {
			handler.OnAdd(obj)
		}
		return nil
	}
	return fmt.Errorf("Resource %s not recognized", resource)
}

func (s *resourceStore) Resources() []ccapi.ResourceType {
	keys := make([]ccapi.ResourceType, 0, len(s.resourceToCache))
	for key := range s.resourceToCache {
		keys = append(keys, key)
	}
	return keys
}

func NewResourceStore() *resourceStore {

	resourceStore := &resourceStore{
		PodCache:     cache.NewStore(cache.MetaNamespaceKeyFunc),
		NodeCache:    cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCache:      cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCCache:     cache.NewStore(cache.MetaNamespaceKeyFunc),
		ServiceCache: cache.NewStore(cache.MetaNamespaceKeyFunc),
		eventHandler: make(map[ccapi.ResourceType]cache.ResourceEventHandler),
	}

	resourceToCache := map[ccapi.ResourceType]cache.Store{
		ccapi.Pods:                   resourceStore.PodCache,
		ccapi.PersistentVolumeClaims: resourceStore.PVCCache,
		ccapi.Nodes:                  resourceStore.NodeCache,
		ccapi.PersistentVolumes:      resourceStore.PVCache,
		ccapi.Services:               resourceStore.ServiceCache,
	}

	resourceStore.resourceToCache = resourceToCache

	return resourceStore
}

func NewResourceReflectors(client clientset.Interface, stopCh <-chan struct{}) *resourceStore {
	rs := NewResourceStore()
	for _, resource := range rs.Resources() {
		listWatcher := cache.NewListWatchFromClient(client.Core().RESTClient(), resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))
		cache.NewReflector(listWatcher, resource.ObjectType(), rs.resourceToCache[resource], 0).Run(stopCh)
	}
	return rs
}
