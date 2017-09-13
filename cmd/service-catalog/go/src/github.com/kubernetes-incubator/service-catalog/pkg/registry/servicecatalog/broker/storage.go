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

package broker

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
	errNotAServiceBroker = errors.New("not a broker")
)

// NewSingular returns a new shell of a service broker, according to the given namespace and
// name
func NewSingular(ns, name string) runtime.Object {
	return &servicecatalog.ServiceBroker{
		TypeMeta: metav1.TypeMeta{
			Kind: tpr.ServiceBrokerKind.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// EmptyObject returns an empty broker
func EmptyObject() runtime.Object {
	return &servicecatalog.ServiceBroker{}
}

// NewList returns a new shell of a broker list
func NewList() runtime.Object {
	return &servicecatalog.ServiceBrokerList{
		TypeMeta: metav1.TypeMeta{
			Kind: tpr.ServiceBrokerListKind.String(),
		},
		Items: []servicecatalog.ServiceBroker{},
	}
}

// CheckObject returns a non-nil error if obj is not a broker object
func CheckObject(obj runtime.Object) error {
	_, ok := obj.(*servicecatalog.ServiceBroker)
	if !ok {
		return errNotAServiceBroker
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
func toSelectableFields(broker *servicecatalog.ServiceBroker) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&broker.ObjectMeta, true)
	return generic.MergeFieldsSets(objectMetaFieldsSet, nil)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	broker, ok := obj.(*servicecatalog.ServiceBroker)
	if !ok {
		return nil, nil, false, fmt.Errorf("given object is not a ServiceBroker")
	}
	return labels.Set(broker.ObjectMeta.Labels), toSelectableFields(broker), broker.Initializers != nil, nil
}

// NewStorage creates a new rest.Storage responsible for accessing ServiceInstance
// resources
func NewStorage(opts server.Options) (brokers, brokersStatus rest.Storage) {
	prefix := "/" + opts.ResourcePrefix()

	storageInterface, dFunc := opts.GetStorage(
		1000,
		&servicecatalog.ServiceBroker{},
		prefix,
		brokerRESTStrategies,
		NewList,
		nil,
		storage.NoTriggerPublisher,
	)

	store := registry.Store{
		NewFunc:     EmptyObject,
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
		QualifiedResource: api.Resource("servicebrokers"),

		CreateStrategy:          brokerRESTStrategies,
		UpdateStrategy:          brokerRESTStrategies,
		DeleteStrategy:          brokerRESTStrategies,
		EnableGarbageCollection: true,

		Storage:     storageInterface,
		DestroyFunc: dFunc,
	}

	statusStore := store
	statusStore.UpdateStrategy = brokerStatusUpdateStrategy

	return &store, &statusStore
}
