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
	errNotABroker = errors.New("not a broker")
)

// NewSingular returns a new shell of a service broker, according to the given namespace and
// name
func NewSingular(ns, name string) runtime.Object {
	return &servicecatalog.Broker{
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
	return &servicecatalog.Broker{}
}

// NewList returns a new shell of a broker list
func NewList() runtime.Object {
	return &servicecatalog.BrokerList{
		TypeMeta: metav1.TypeMeta{
			Kind: tpr.ServiceBrokerListKind.String(),
		},
		Items: []servicecatalog.Broker{},
	}
}

// CheckObject returns a non-nil error if obj is not a broker object
func CheckObject(obj runtime.Object) error {
	_, ok := obj.(*servicecatalog.Broker)
	if !ok {
		return errNotABroker
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
func toSelectableFields(broker *servicecatalog.Broker) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&broker.ObjectMeta, true)
	return generic.MergeFieldsSets(objectMetaFieldsSet, nil)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	broker, ok := obj.(*servicecatalog.Broker)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a Broker")
	}
	return labels.Set(broker.ObjectMeta.Labels), toSelectableFields(broker), nil
}

// NewStorage creates a new rest.Storage responsible for accessing Instance
// resources
func NewStorage(opts server.Options) (brokers, brokersStatus rest.Storage) {
	prefix := "/" + opts.ResourcePrefix()

	storageInterface, dFunc := opts.GetStorage(
		1000,
		&servicecatalog.Broker{},
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
		QualifiedResource: api.Resource("brokers"),

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
