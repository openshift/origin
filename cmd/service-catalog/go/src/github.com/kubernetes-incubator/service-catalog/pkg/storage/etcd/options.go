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

package etcd

import (
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
)

// Options is the set of options necessary for creating etcd-backed storage
type Options struct {
	RESTOptions   generic.RESTOptions
	Capacity      int
	ObjectType    runtime.Object
	ScopeStrategy rest.NamespaceScopedStrategy
	NewListFunc   func() runtime.Object
	GetAttrsFunc  func(runtime.Object) (labels.Set, fields.Set, bool, error)
	Trigger       storage.TriggerPublisherFunc
}
