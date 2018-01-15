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
	"reflect"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
)

type FakeResourceStore struct {
	PodsData                   func() []*v1.Pod
	ServicesData               func() []*v1.Service
	NodesData                  func() []*v1.Node
	PersistentVolumesData      func() []*v1.PersistentVolume
	PersistentVolumeClaimsData func() []*v1.PersistentVolumeClaim
	// TODO(jchaloup): fill missing resource functions
}

func (s *FakeResourceStore) Add(resource ccapi.ResourceType, obj interface{}) error {
	return nil
}

func (s *FakeResourceStore) Update(resource ccapi.ResourceType, obj interface{}) error {
	return nil
}

func (s *FakeResourceStore) Delete(resource ccapi.ResourceType, obj interface{}) error {
	return nil
}

func resourcesToItems(objs interface{}) []interface{} {
	objsSlice := reflect.ValueOf(objs)
	items := make([]interface{}, 0, objsSlice.Len())
	for i := 0; i < objsSlice.Len(); i++ {
		items = append(items, objsSlice.Index(i).Interface())
	}
	return items
}

func findResource(obj interface{}, objs interface{}) (item interface{}, exists bool, err error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return nil, false, err
	}

	var obj_key string
	var key_err error
	objsSlice := reflect.ValueOf(objs)
	for i := 0; i < objsSlice.Len(); i++ {
		item := objsSlice.Index(i).Interface()
		// TODO(jchaloup): make this resource type independent
		switch item.(type) {
		case v1.Pod:
			value := item.(v1.Pod)
			obj_key, key_err = cache.MetaNamespaceKeyFunc(metav1.Object(&value))
		case v1.Service:
			value := item.(v1.Service)
			obj_key, key_err = cache.MetaNamespaceKeyFunc(metav1.Object(&value))
		case v1.Node:
			value := item.(v1.Node)
			obj_key, key_err = cache.MetaNamespaceKeyFunc(metav1.Object(&value))
		case v1.PersistentVolume:
			value := item.(v1.PersistentVolume)
			obj_key, key_err = cache.MetaNamespaceKeyFunc(metav1.Object(&value))
		case v1.PersistentVolumeClaim:
			value := item.(v1.PersistentVolumeClaim)
			obj_key, key_err = cache.MetaNamespaceKeyFunc(metav1.Object(&value))
		}
		if key_err != nil {
			return nil, false, key_err
		}
		if obj_key == key {
			return item, true, nil
		}
	}
	return nil, false, fmt.Errorf("Resource obj not found")
}

func (s *FakeResourceStore) List(resource ccapi.ResourceType) []interface{} {
	switch resource {
	case ccapi.Pods:
		if s.PodsData == nil {
			return make([]interface{}, 0, 0)
		}
		return resourcesToItems(s.PodsData())
	case ccapi.Services:
		if s.ServicesData == nil {
			return make([]interface{}, 0, 0)
		}
		return resourcesToItems(s.ServicesData())
	case ccapi.Nodes:
		if s.NodesData == nil {
			return make([]interface{}, 0, 0)
		}
		return resourcesToItems(s.NodesData())
	case ccapi.PersistentVolumes:
		if s.PersistentVolumesData == nil {
			return make([]interface{}, 0, 0)
		}
		return resourcesToItems(s.PersistentVolumesData())
	case ccapi.PersistentVolumeClaims:
		if s.PersistentVolumeClaimsData == nil {
			return make([]interface{}, 0, 0)
		}
		return resourcesToItems(s.PersistentVolumeClaimsData())
	}
	return make([]interface{}, 0, 0)
}

func (s *FakeResourceStore) Get(resource ccapi.ResourceType, obj interface{}) (item interface{}, exists bool, err error) {
	switch resource {
	case ccapi.Pods:
		return findResource(obj, s.PodsData())
	case ccapi.Services:
		return findResource(obj, s.ServicesData())
	case ccapi.Nodes:
		return findResource(obj, s.NodesData())
	case ccapi.PersistentVolumes:
		return findResource(obj, s.PersistentVolumesData())
	case ccapi.PersistentVolumeClaims:
		return findResource(obj, s.PersistentVolumeClaimsData())
	}
	return nil, false, nil
}

func (s *FakeResourceStore) GetByKey(resource ccapi.ResourceType, key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (s *FakeResourceStore) RegisterEventHandler(resource ccapi.ResourceType, handler cache.ResourceEventHandler) error {
	return nil
}

func (s *FakeResourceStore) Replace(resource ccapi.ResourceType, items []interface{}, resourceVersion string) error {
	return nil
}

func (s *FakeResourceStore) Resources() []ccapi.ResourceType {
	return []ccapi.ResourceType{ccapi.Pods, ccapi.Services, ccapi.Nodes, ccapi.PersistentVolumes, ccapi.PersistentVolumeClaims}
}
