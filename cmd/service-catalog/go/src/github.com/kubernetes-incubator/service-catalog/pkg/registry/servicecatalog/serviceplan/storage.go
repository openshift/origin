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

package serviceplan

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
	errNotAServicePlan = errors.New("not a servicePlan")
)

// NewSingular returns a new shell of a service servicePlan, according to the given namespace and
// name
func NewSingular(ns, name string) runtime.Object {
	return &servicecatalog.ServicePlan{
		TypeMeta: metav1.TypeMeta{
			Kind: "ServicePlan",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// EmptyObject returns an empty servicePlan
func EmptyObject() runtime.Object {
	return &servicecatalog.ServicePlan{}
}

// NewList returns a new shell of a servicePlan list
func NewList() runtime.Object {
	return &servicecatalog.ServicePlanList{
		TypeMeta: metav1.TypeMeta{
			Kind: "ServicePlanList",
		},
		Items: []servicecatalog.ServicePlan{},
	}
}

// CheckObject returns a non-nil error if obj is not a servicePlan object
func CheckObject(obj runtime.Object) error {
	_, ok := obj.(*servicecatalog.ServicePlan)
	if !ok {
		return errNotAServicePlan
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
func toSelectableFields(servicePlan *servicecatalog.ServicePlan) fields.Set {
	spSpecificFieldsSet := make(fields.Set, 4)
	spSpecificFieldsSet["spec.serviceBrokerName"] = servicePlan.Spec.ServiceBrokerName
	spSpecificFieldsSet["spec.serviceClassRef.name"] = servicePlan.Spec.ServiceClassRef.Name
	spSpecificFieldsSet["spec.externalName"] = servicePlan.Spec.ExternalName
	spSpecificFieldsSet["spec.externalID"] = servicePlan.Spec.ExternalID
	return generic.AddObjectMetaFieldsSet(spSpecificFieldsSet, &servicePlan.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	servicePlan, ok := obj.(*servicecatalog.ServicePlan)
	if !ok {
		return nil, nil, false, fmt.Errorf("given object is not a ServicePlan")
	}
	return labels.Set(servicePlan.ObjectMeta.Labels), toSelectableFields(servicePlan), servicePlan.Initializers != nil, nil
}

// NewStorage creates a new rest.Storage responsible for accessing Instance
// resources
func NewStorage(opts server.Options) rest.Storage {
	prefix := "/" + opts.ResourcePrefix()

	storageInterface, dFunc := opts.GetStorage(
		1000,
		&servicecatalog.ServicePlan{},
		prefix,
		serviceplanRESTStrategies,
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
		QualifiedResource: coreapi.Resource("serviceplans"),

		CreateStrategy: serviceplanRESTStrategies,
		UpdateStrategy: serviceplanRESTStrategies,
		DeleteStrategy: serviceplanRESTStrategies,
		Storage:        storageInterface,
		DestroyFunc:    dFunc,
	}

	return &store
}
