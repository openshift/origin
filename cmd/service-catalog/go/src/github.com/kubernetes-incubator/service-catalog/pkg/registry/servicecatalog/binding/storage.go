/*
Copyright 2016 The Kubernetes Authors.

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

package binding

import (
	"errors"
	"fmt"

	scmeta "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/kubernetes-incubator/service-catalog/pkg/storage/tpr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/client-go/pkg/api"
)

var (
	errNotABinding = errors.New("not a binding")
)

// NewSingular returns a new shell of a service binding, according to the given namespace and
// name
func NewSingular(ns, name string) runtime.Object {
	return &servicecatalog.Binding{
		TypeMeta: metav1.TypeMeta{
			Kind: tpr.ServiceBindingKind.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// EmptyObject returns an empty binding
func EmptyObject() runtime.Object {
	return &servicecatalog.Binding{}
}

// NewList returns a new shell of a binding list
func NewList() runtime.Object {
	return &servicecatalog.BindingList{
		TypeMeta: metav1.TypeMeta{
			Kind: tpr.ServiceBindingListKind.String(),
		},
		Items: []servicecatalog.Binding{},
	}
}

// CheckObject returns a non-nil error if obj is not a binding object
func CheckObject(obj runtime.Object) error {
	_, ok := obj.(*servicecatalog.Binding)
	if !ok {
		return errNotABinding
	}
	return nil
}

// Match determines whether an Instance matches a field and label
// selector.
func Match(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// toSelectableFields returns a field set that represents the object for matching purposes.
func toSelectableFields(binding *servicecatalog.Binding) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&binding.ObjectMeta, true)
	return generic.MergeFieldsSets(objectMetaFieldsSet, nil)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	binding, ok := obj.(*servicecatalog.Binding)
	if !ok {
		return nil, nil, false, fmt.Errorf("given object is not a Binding")
	}
	return labels.Set(binding.ObjectMeta.Labels), toSelectableFields(binding), binding.Initializers != nil, nil
}

// NewStorage creates a new rest.Storage responsible for accessing Instance
// resources
func NewStorage(opts server.Options) (rest.Storage, rest.Storage, error) {
	prefix := "/" + opts.ResourcePrefix()

	storageInterface, dFunc := opts.GetStorage(
		1000,
		&servicecatalog.Binding{},
		prefix,
		bindingRESTStrategies,
		NewList,
		nil,
		storage.NoTriggerPublisher,
	)

	store := registry.Store{
		NewFunc: EmptyObject,
		// NewListFunc returns an object capable of storing results of an etcd list.
		NewListFunc: NewList,
		KeyRootFunc: opts.KeyRootFunc(),
		KeyFunc:     opts.KeyFunc(true),
		// Retrieve the name field of the resource.
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return scmeta.GetAccessor().Name(obj)
		},
		// Used to match objects based on labels/fields for list.
		PredicateFunc: Match,
		// QualifiedResource should always be plural
		QualifiedResource: api.Resource("bindings"),

		CreateStrategy:          bindingRESTStrategies,
		UpdateStrategy:          bindingRESTStrategies,
		DeleteStrategy:          bindingRESTStrategies,
		EnableGarbageCollection: true,

		Storage:     storageInterface,
		DestroyFunc: dFunc,
	}

	statusStore := store
	statusStore.UpdateStrategy = bindingStatusUpdateStrategy

	return &store, &statusStore, nil
}
