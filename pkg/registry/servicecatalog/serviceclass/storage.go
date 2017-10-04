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

package serviceclass

import (
	"errors"
	"fmt"

	scmeta "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	coreapi "k8s.io/client-go/pkg/api"
)

var (
	errNotAServiceClass = errors.New("not a service class")
)

// NewSingular returns a new shell of a service class, according to the given namespace and
// name
func NewSingular(ns, name string) runtime.Object {
	return &servicecatalog.ServiceClass{
		TypeMeta: metav1.TypeMeta{
			Kind: "ServiceClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// EmptyObject returns an empty service class
func EmptyObject() runtime.Object {
	return &servicecatalog.ServiceClass{}
}

// NewList returns a new shell of a service class list
func NewList() runtime.Object {
	return &servicecatalog.ServiceClassList{
		TypeMeta: metav1.TypeMeta{
			Kind: "ServiceClassList",
		},
		Items: []servicecatalog.ServiceClass{},
	}
}

// CheckObject returns a non-nil error if obj is not a service class object
func CheckObject(obj runtime.Object) error {
	_, ok := obj.(*servicecatalog.ServiceClass)
	if !ok {
		return errNotAServiceClass
	}
	return nil
}

// Match determines whether an ServiceInstance matches a field and label
// selector.
func Match(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// toSelectableFields returns a field set that represents the object for matching purposes.
func toSelectableFields(serviceClass *servicecatalog.ServiceClass) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	scSpecificFieldsSet := make(fields.Set, 3)
	scSpecificFieldsSet["spec.serviceBrokerName"] = serviceClass.Spec.ServiceBrokerName
	scSpecificFieldsSet["spec.externalName"] = serviceClass.Spec.ExternalName
	scSpecificFieldsSet["spec.externalID"] = serviceClass.Spec.ExternalID
	return generic.AddObjectMetaFieldsSet(scSpecificFieldsSet, &serviceClass.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	serviceclass, ok := obj.(*servicecatalog.ServiceClass)
	if !ok {
		return nil, nil, false, fmt.Errorf("given object is not a ServiceClass")
	}
	return labels.Set(serviceclass.ObjectMeta.Labels), toSelectableFields(serviceclass), serviceclass.Initializers != nil, nil
}

// NewStorage creates a new rest.Storage responsible for accessing ServiceClass
// resources
func NewStorage(opts server.Options) rest.Storage {
	prefix := "/" + opts.ResourcePrefix()

	storageInterface, dFunc := opts.GetStorage(
		1000,
		&servicecatalog.ServiceClass{},
		prefix,
		serviceclassRESTStrategies,
		NewList,
		nil,
		storage.NoTriggerPublisher,
	)

	store := registry.Store{
		NewFunc: EmptyObject,
		// NewListFunc returns an object capable of storing results of an etcd list.
		NewListFunc: NewList,
		KeyRootFunc: opts.KeyRootFunc(),
		KeyFunc:     opts.KeyFunc(false),
		// Retrieve the name field of the resource.
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return scmeta.GetAccessor().Name(obj)
		},
		// Used to match objects based on labels/fields for list.
		PredicateFunc: Match,
		// QualifiedResource should always be plural
		QualifiedResource: coreapi.Resource("serviceclasses"),

		CreateStrategy: serviceclassRESTStrategies,
		UpdateStrategy: serviceclassRESTStrategies,
		DeleteStrategy: serviceclassRESTStrategies,
		Storage:        storageInterface,
		DestroyFunc:    dFunc,
	}

	return &store
}
