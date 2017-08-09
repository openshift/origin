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

package tpr

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	scmeta "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/rest/core/fake"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/etcd"
)

const (
	globalNamespace = "globalns"
	namespace       = "testns"
	name            = "testthing"
)

func TestCreateExistingWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	// Ensure an existing broker
	fakeCl.Storage.Set(globalNamespace, ServiceBrokerKind.URLName(), name, &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})
	inputBroker := &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	createdBroker := &sc.Broker{}
	err = iface.Create(
		context.Background(),
		key,
		inputBroker,
		createdBroker,
		uint64(0),
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyExists); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	if err = deepCompare(
		"output",
		createdBroker,
		"new broker",
		&sc.Broker{},
	); err != nil {
		t.Fatal(err)
	}
}

func TestCreateExistingWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	// Ensure an existing instance
	fakeCl.Storage.Set(namespace, ServiceInstanceKind.URLName(), name, &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	})
	inputInstance := &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	createdInstance := &sc.Instance{}
	err = iface.Create(
		context.Background(),
		key,
		inputInstance,
		createdInstance,
		uint64(0),
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyExists); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new instance
	if err = deepCompare(
		"output",
		createdInstance,
		"new instance",
		&sc.Instance{},
	); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	inputBroker := &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: sc.BrokerSpec{
			URL: "http://my-awesome-broker.io",
		},
	}
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	createdBroker := &sc.Broker{}
	if err := iface.Create(
		context.Background(),
		key,
		inputBroker,
		createdBroker,
		uint64(0),
	); err != nil {
		t.Fatalf("error on create (%s)", err)
	}
	// Confirm resource version got set during the create operation
	if createdBroker.ResourceVersion == "" {
		t.Fatalf("resource version was not set as expected")
	}
	// Confirm the output is identical to what is in storage (nothing funny
	// happened during encoding / decoding the response).
	obj := fakeCl.Storage.Get(globalNamespace, ServiceBrokerKind.URLName(), name)
	if obj == nil {
		t.Fatal("no broker was in storage")
	}
	err = deepCompare("output", createdBroker, "object in storage", obj)
	if err != nil {
		t.Fatal(err)
	}
	// Output and what's in storage should be known to be deeply equal at this
	// point. Compare either of those to what was passed in. The only diff should
	// be resource version, so we will set that first.
	inputBroker.ResourceVersion = createdBroker.ResourceVersion
	err = deepCompare("input", inputBroker, "output", createdBroker)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	inputInstance := &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: sc.InstanceSpec{
			ExternalID: "e6a8edad-145a-47f1-aaba-c0eb20b233a3",
			PlanName:   "some-awesome-plan",
		},
	}
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	createdInstance := &sc.Instance{}
	if err := iface.Create(
		context.Background(),
		key,
		inputInstance,
		createdInstance,
		uint64(0),
	); err != nil {
		t.Fatalf("error on create (%s)", err)
	}
	// Confirm resource version got set during the create operation
	if createdInstance.ResourceVersion == "" {
		t.Fatalf("resource version was not set as expected")
	}
	// Confirm the output is identical to what is in storage (nothing funny
	// happened during encoding / decoding the response).
	obj := fakeCl.Storage.Get(namespace, ServiceInstanceKind.URLName(), name)
	if obj == nil {
		t.Fatal("no instance was in storage")
	}
	err = deepCompare("output", createdInstance, "object in storage", obj)
	if err != nil {
		t.Fatal(err)
	}
	// Output and what's in storage should be known to be deeply equal at this
	// point. Compare either of those to what was passed in. The only diff should
	// be resource version, so we will set that first.
	inputInstance.ResourceVersion = createdInstance.ResourceVersion
	err = deepCompare("input", inputInstance, "output", createdInstance)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNonExistentWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	outBroker := &sc.Broker{}
	// Ignore not found
	if err := iface.Get(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		outBroker,
		true,
	); err != nil {
		t.Fatalf("expected no error, but received one (%s)", err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	err = deepCompare("output", outBroker, "new broker", &sc.Broker{})
	if err != nil {
		t.Fatal(err)
	}
	// Do not ignore not found
	err = iface.Get(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		outBroker,
		false,
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyNotFound); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	err = deepCompare("output", outBroker, "new broker", &sc.Broker{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNonExistentWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	outInstance := &sc.Instance{}
	// Ignore not found
	if err := iface.Get(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		outInstance,
		true,
	); err != nil {
		t.Fatalf("expected no error, but received one (%s)", err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new instance
	err = deepCompare("output", outInstance, "new instance", &sc.Instance{})
	if err != nil {
		t.Fatal(err)
	}
	// Do not ignore not found
	err = iface.Get(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		outInstance,
		false,
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyNotFound); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new instance
	err = deepCompare("output", outInstance, "new instance", &sc.Instance{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	// Ensure an existing broker
	fakeCl.Storage.Set(globalNamespace, ServiceBrokerKind.URLName(), name, &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	broker := &sc.Broker{}
	if err := iface.Get(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		broker,
		false, // Do not ignore if not found; error instead
	); err != nil {
		t.Fatalf("error getting object (%s)", err)
	}
	// Confirm the output is identical to what is in storage (nothing funny
	// happened during encoding / decoding the response).
	obj := fakeCl.Storage.Get(globalNamespace, ServiceBrokerKind.URLName(), name)
	if obj == nil {
		t.Fatal("no broker was in storage")
	}
	err = deepCompare("output", broker, "object in storage", obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	// Ensure an existing instance
	fakeCl.Storage.Set(namespace, ServiceInstanceKind.URLName(), name, &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: sc.InstanceSpec{
			ExternalID: "e6a8edad-145a-47f1-aaba-c0eb20b233a3",
			PlanName:   "some-awesome-plan",
		},
	})
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	instance := &sc.Instance{}
	if err := iface.Get(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		instance,
		false, // Do not ignore if not found; error instead
	); err != nil {
		t.Fatalf("error getting object (%s)", err)
	}
	// Confirm the output is identical to what is in storage (nothing funny
	// happened during encoding / decoding the response).
	obj := fakeCl.Storage.Get(namespace, ServiceInstanceKind.URLName(), name)
	if obj == nil {
		t.Fatal("no instance was in storage")
	}
	err = deepCompare("output", instance, "object in storage", obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetEmptyListWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.BrokerList{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	key := keyer.KeyRoot(request.NewContext())
	outBrokerList := &sc.BrokerList{}
	if err := iface.List(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		// TODO: Current impl ignores selection predicate-- may be wrong
		storage.SelectionPredicate{},
		outBrokerList,
	); err != nil {
		t.Fatalf("error listing objects (%s)", err)
	}
	if len(outBrokerList.Items) != 0 {
		t.Fatalf(
			"expected an empty list, but got %d items",
			len(outBrokerList.Items),
		)
	}
	// Repeat using GetToList
	if err := iface.GetToList(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		// TODO: Current impl ignores selection predicate-- may be wrong
		storage.SelectionPredicate{},
		outBrokerList,
	); err != nil {
		t.Fatalf("error listing objects (%s)", err)
	}
	if len(outBrokerList.Items) != 0 {
		t.Fatalf(
			"expected an empty list, but got %d items",
			len(outBrokerList.Items),
		)
	}
}

func TestGetEmptyListWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.InstanceList{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key := keyer.KeyRoot(ctx)
	outInstanceList := &sc.InstanceList{}
	if err := iface.List(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		// TODO: Current impl ignores selection predicate-- may be wrong
		storage.SelectionPredicate{},
		outInstanceList,
	); err != nil {
		t.Fatalf("error listing objects (%s)", err)
	}
	if len(outInstanceList.Items) != 0 {
		t.Fatalf(
			"expected an empty list, but got %d items",
			len(outInstanceList.Items),
		)
	}
	// Repeat using GetToList
	if err := iface.GetToList(
		context.Background(),
		key,
		"", // TODO: Current impl ignores resource version-- may be wrong
		// TODO: Current impl ignores selection predicate-- may be wrong
		storage.SelectionPredicate{},
		outInstanceList,
	); err != nil {
		t.Fatalf("error listing objects (%s)", err)
	}
	if len(outInstanceList.Items) != 0 {
		t.Fatalf(
			"expected an empty list, but got %d items",
			len(outInstanceList.Items),
		)
	}
}

func TestGetListWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.BrokerList{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	// Ensure an existing broker
	fakeCl.Storage.Set(globalNamespace, ServiceBrokerKind.URLName(), name, &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})
	list := &sc.BrokerList{}
	if err := iface.List(
		context.Background(),
		keyer.KeyRoot(request.NewContext()),
		"", // TODO: Current impl ignores resource version-- may be wrong
		// TODO: Current impl ignores selection predicate-- may be wrong
		storage.SelectionPredicate{},
		list,
	); err != nil {
		t.Fatalf("error listing objects (%s)", err)
	}
	// List should contain precisely one item
	if len(list.Items) != 1 {
		t.Fatalf(
			"expected list to contain exactly one item, but got %d items",
			len(list.Items),
		)
	}
	// That one list item should be deeply equal to what's in storage
	obj := fakeCl.Storage.Get(globalNamespace, ServiceBrokerKind.URLName(), name)
	if obj == nil {
		t.Fatal("no broker was in storage")
	}
	if err := deepCompare(
		"retrieved list item",
		&list.Items[0],
		"object in storage",
		obj,
	); err != nil {
		t.Fatal(err)
	}
}

func TestGetListWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.InstanceList{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	// Ensure an existing instance
	fakeCl.Storage.Set(globalNamespace, ServiceInstanceKind.URLName(), name, &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: sc.InstanceSpec{
			ExternalID: "e6a8edad-145a-47f1-aaba-c0eb20b233a3",
			PlanName:   "some-awesome-plan",
		},
	})
	list := &sc.InstanceList{}
	if err := iface.List(
		context.Background(),
		keyer.KeyRoot(request.NewContext()),
		"", // TODO: Current impl ignores resource version-- may be wrong
		// TODO: Current impl ignores selection predicate-- may be wrong
		storage.SelectionPredicate{},
		list,
	); err != nil {
		t.Fatalf("error listing objects (%s)", err)
	}
	// List should contain precisely one item
	if len(list.Items) != 1 {
		t.Fatalf(
			"expected list to contain exactly one item, but got %d items",
			len(list.Items),
		)
	}
	// That one list item should be deeply equal to what's in storage
	obj := fakeCl.Storage.Get(globalNamespace, ServiceInstanceKind.URLName(), name)
	if obj == nil {
		t.Fatal("no instance was in storage")
	}
	if err := deepCompare(
		"retrieved list item",
		&list.Items[0],
		"object in storage",
		obj,
	); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNonExistentWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	key, err := keyer.Key(request.NewContext(), name)
	newURL := "http://your-incredible-broker.io"
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	updatedBroker := &sc.Broker{}
	// Ignore not found
	err = iface.GuaranteedUpdate(
		context.Background(),
		key,
		updatedBroker,
		true, // Ignore not found
		nil,  // No preconditions for the update
		storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
			broker := obj.(*sc.Broker)
			broker.Spec.URL = newURL
			return broker, nil
		}),
	)
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	err = deepCompare("updated broker", updatedBroker, "new broker", &sc.Broker{})
	if err != nil {
		t.Fatal(err)
	}
	// Do not ignore not found
	err = iface.GuaranteedUpdate(
		context.Background(),
		key,
		updatedBroker,
		false, // Do not ignore not found
		nil,   // No preconditions for the update
		storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
			broker := obj.(*sc.Broker)
			broker.Spec.URL = newURL
			return broker, nil
		}),
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyNotFound); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	err = deepCompare("updated broker", updatedBroker, "new broker", &sc.Broker{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNonExistentWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	newPlanName := "my-really-awesome-plan"
	updatedInstance := &sc.Instance{}
	// Ignore not found
	err = iface.GuaranteedUpdate(
		context.Background(),
		key,
		updatedInstance,
		true, // Ignore not found
		nil,  // No preconditions for the update
		storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
			instance := obj.(*sc.Instance)
			instance.Spec.PlanName = newPlanName
			return instance, nil
		}),
	)
	// Object should remain unmodified-- i.e. deeply equal to a new instance
	err = deepCompare("updated instance", updatedInstance, "new instance", &sc.Instance{})
	if err != nil {
		t.Fatal(err)
	}
	// Do not ignore not found
	err = iface.GuaranteedUpdate(
		context.Background(),
		key,
		updatedInstance,
		false, // Do not ignore not found
		nil,   // No preconditions for the update
		storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
			instance := obj.(*sc.Instance)
			instance.Spec.PlanName = newPlanName
			return instance, nil
		}),
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyNotFound); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	err = deepCompare("updated instance", updatedInstance, "new broker", &sc.Instance{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	var origRev uint64 = 1
	newURL := "http://your-incredible-broker.io"
	fakeCl.Storage.Set(globalNamespace, ServiceBrokerKind.URLName(), name, &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: fmt.Sprintf("%d", origRev),
		},
		Spec: sc.BrokerSpec{
			URL: "http://my-awesome-broker.io",
		},
	})
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	updatedBroker := &sc.Broker{}
	err = iface.GuaranteedUpdate(
		context.Background(),
		key,
		updatedBroker,
		false, // Don't ignore not found
		nil,   // No preconditions for the update
		storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
			broker := obj.(*sc.Broker)
			broker.Spec.URL = newURL
			return broker, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error updating object (%s)", err)
	}
	updatedRev, err := iface.versioner.ObjectResourceVersion(updatedBroker)
	if err != nil {
		t.Fatalf("error extracting resource version (%s)", err)
	}
	if updatedRev <= origRev {
		t.Fatalf(
			"expected a new resource version > %d; got %d",
			origRev,
			updatedRev,
		)
	}
	if updatedBroker.Spec.URL != newURL {
		t.Fatal("expectd url to have been updated, but it was not")
	}
}

func TestUpdateWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	var origRev uint64 = 1
	newPlanName := "my-really-awesome-plan"
	fakeCl.Storage.Set(namespace, ServiceInstanceKind.URLName(), name, &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: fmt.Sprintf("%d", origRev),
		},
		Spec: sc.InstanceSpec{
			PlanName: "my-awesome-plan",
		},
	})
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	updatedInstance := &sc.Instance{}
	err = iface.GuaranteedUpdate(
		context.Background(),
		key,
		updatedInstance,
		false, // Don't ignore not found
		nil,   // No preconditions for the update
		storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
			instance := obj.(*sc.Instance)
			instance.Spec.PlanName = newPlanName
			return instance, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error updating object (%s)", err)
	}
	updatedRev, err := iface.versioner.ObjectResourceVersion(updatedInstance)
	if err != nil {
		t.Fatalf("error extracting resource version (%s)", err)
	}
	if updatedRev <= origRev {
		t.Fatalf(
			"expected a new resource version > %d; got %d",
			origRev,
			updatedRev,
		)
	}
	if updatedInstance.Spec.PlanName != newPlanName {
		t.Fatal("expectd plan name to have been updated, but it was not")
	}
}

func TestDeleteNonExistentWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	outBroker := &sc.Broker{}
	err = iface.Delete(
		context.Background(),
		key,
		outBroker,
		nil, // TODO: Current impl ignores preconditions-- may be wrong
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyNotFound); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new broker
	err = deepCompare("output", outBroker, "new broker", &sc.Broker{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteNonExistentWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	outInstance := &sc.Instance{}
	err = iface.Delete(
		context.Background(),
		key,
		outInstance,
		nil, // TODO: Current impl ignores preconditions-- may be wrong
	)
	if err = verifyStorageError(err, storage.ErrCodeKeyNotFound); err != nil {
		t.Fatal(err)
	}
	// Object should remain unmodified-- i.e. deeply equal to a new instance
	err = deepCompare("output", outInstance, "new instance", &sc.Instance{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	var origRev uint64 = 1
	brokerNoFinalizers := &sc.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: fmt.Sprintf("%d", origRev),
		},
	}
	brokerWithFinalizers := *brokerNoFinalizers
	brokerWithFinalizers.Finalizers = append(brokerWithFinalizers.Finalizers, v1alpha1.FinalizerServiceCatalog)
	fakeCl.Storage.Set(globalNamespace, ServiceBrokerKind.URLName(), name, &brokerWithFinalizers)
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	outBroker := &sc.Broker{}
	err = iface.Delete(
		context.Background(),
		key,
		outBroker,
		nil, // TODO: Current impl ignores preconditions-- may be wrong
	)
	if err != nil {
		t.Fatalf("unexpected error deleting object (%s)", err)
	}
	// Object should be removed from underlying storage
	obj := fakeCl.Storage.Get(globalNamespace, ServiceBrokerKind.URLName(), name)
	finalizers, err := scmeta.GetFinalizers(obj)
	if err != nil {
		t.Fatalf("error getting finalizers (%s)", err)
	}
	if len(finalizers) != 0 {
		t.Fatalf("expected no finalizers, got %#v", finalizers)
	}
	// the delete call does a PUT, which increments the resource version. brokerNoFinalizers
	// and obj should match exactly except for the resource version, so do the increment here
	brokerNoFinalizers.ResourceVersion = fmt.Sprintf("%d", origRev+1)
	if err := deepCompare("expected", brokerNoFinalizers, "actual", obj); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	var origRev uint64 = 1
	instanceNoFinalizers := &sc.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: fmt.Sprintf("%d", origRev),
		},
		Spec: sc.InstanceSpec{
			ExternalID: "76026cec-f601-487f-b6bd-6d6f8240d620",
		},
	}
	instanceWithFinalizers := *instanceNoFinalizers
	instanceWithFinalizers.Finalizers = append(instanceWithFinalizers.Finalizers, v1alpha1.FinalizerServiceCatalog)
	fakeCl.Storage.Set(namespace, ServiceInstanceKind.URLName(), name, &instanceWithFinalizers)
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, namespace)
	key, err := keyer.Key(ctx, name)
	if err != nil {
		t.Fatalf("error constructing key (%s)", err)
	}
	outInstance := &sc.Instance{}
	err = iface.Delete(
		context.Background(),
		key,
		outInstance,
		nil, // TODO: Current impl ignores preconditions-- may be wrong
	)
	if err != nil {
		t.Fatalf("unexpected error deleting object (%s)", err)
	}
	// Object should be removed from underlying storage
	obj := fakeCl.Storage.Get(namespace, ServiceInstanceKind.URLName(), name)
	finalizers, err := scmeta.GetFinalizers(obj)
	if err != nil {
		t.Fatalf("error getting finalizers (%s)", err)
	}
	if len(finalizers) != 0 {
		t.Fatalf("expected no finalizers, got %#v", finalizers)
	}
	// the delete call does a PUT, which increments the resource version. brokerNoFinalizers
	// and obj should match exactly except for the resource version, so do the increment here
	instanceNoFinalizers.ResourceVersion = fmt.Sprintf("%d", origRev+1)
	if err := deepCompare("expected", instanceNoFinalizers, "actual", obj); err != nil {
		t.Fatal(err)
	}
}

func TestWatchWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Instance{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)
	obj := &sc.Instance{
		TypeMeta:   metav1.TypeMeta{Kind: ServiceInstanceKind.String()},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: sc.InstanceSpec{
			ExternalID: "9ac07e7d-6c32-48f6-96ef-5a215f69df36",
		},
	}
	// send an unversioned object into the watch test. it sends this object to the
	// fake REST client, which encodes the unversioned object into bytes & sends them
	// to the storage interface's client. The storage interface's watchFilterer
	// function calls singularShell to get the object to decode into, and singularShell returns
	// an unversioned object. After watchFilterer decodes into the unversioned object,
	// it simply returns it back to the watch stream
	if err := runWatchTest(keyer, fakeCl, iface, obj); err != nil {
		t.Fatal(err)
	}
}

func TestWatchWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.Broker{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	obj := &sc.Broker{
		TypeMeta:   metav1.TypeMeta{Kind: ServiceBrokerKind.String()},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	// send an unversioned object into the watch test. it sends this object to the
	// fake REST client, which encodes the unversioned object into bytes & sends them
	// to the storage interface's client. The storage interface's watchFilterer
	// function calls singularShell to get the object to decode into, and singularShell returns
	// an unversioned object. After watchFilterer decodes into the unversioned object,
	// it does the necessary processing to strip out the namespace and return the new
	// object back into the watch stream
	if err := runWatchTest(keyer, fakeCl, iface, obj); err != nil {
		t.Fatal(err)
	}
}

func TestWatchListWithNamespace(t *testing.T) {
	keyer := getInstanceKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.InstanceList{}
	})
	iface := getInstanceTPRStorageIFace(t, keyer, fakeCl)

	obj := &sc.InstanceList{
		Items: []sc.Instance{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s1", name),
					Namespace: namespace,
				},
				Spec: sc.InstanceSpec{ExternalID: "b13843f9-aea7-4ef6-b276-771a5ced2c65"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s2", name),
					Namespace: namespace,
				},
				Spec: sc.InstanceSpec{ExternalID: "b23843f9-aea7-4ef6-b276-771a5ced2c65"},
			},
		},
	}
	// send an unversioned object into the watchList test. it sends this object to the
	// fake REST client, which encodes the unversioned object into bytes & sends them
	// to the storage interface's client. The storage interface's watchFilterer
	// function calls listShell to get the object to decode into, and listShell returns
	// an unversioned object. After watchFilterer decodes into the unversioned object,
	// it simply returns it
	if err := runWatchListTest(keyer, fakeCl, iface, obj); err != nil {
		t.Fatal(err)
	}
}

func TestWatchListWithNoNamespace(t *testing.T) {
	keyer := getBrokerKeyer()
	fakeCl := fake.NewRESTClient(func() runtime.Object {
		return &sc.BrokerList{}
	})
	iface := getBrokerTPRStorageIFace(t, keyer, fakeCl)
	obj := &sc.BrokerList{
		Items: []sc.Broker{
			{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s1", name)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s2", name)},
			},
		},
	}
	// send an unversioned object into the watchList test. it sends this object to the
	// fake REST client, which encodes the unversioned object into bytes & sends them
	// to the storage interface's client. The storage interface's watchFilterer
	// function calls listShell to get the object to decode into, and listShell returns
	// an unversioned object. After watchFilterer decodes into the unversioned object,
	// it does necessary processing to strip the namespaces out of each object.
	if err := runWatchListTest(keyer, fakeCl, iface, obj); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveNamespace(t *testing.T) {
	obj := &servicecatalog.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "testns",
		},
	}
	if err := removeNamespace(obj); err != nil {
		t.Fatalf("couldn't remove namespace (%s", err)
	}
	if obj.Namespace != "" {
		t.Fatalf(
			"couldn't remove namespace from object. it is still %s",
			obj.Namespace,
		)
	}
}
func getBrokerKeyer() Keyer {
	return Keyer{
		DefaultNamespace: globalNamespace,
		ResourceName:     ServiceBrokerKind.String(),
		Separator:        "/",
	}
}

func getInstanceKeyer() Keyer {
	return Keyer{
		ResourceName: ServiceInstanceKind.String(),
		Separator:    "/",
	}
}

func getBrokerTPRStorageIFace(
	t *testing.T,
	keyer Keyer,
	restCl *fake.RESTClient,
) *store {
	codec, err := testapi.GetCodecForObject(&sc.Broker{})
	if err != nil {
		t.Fatalf("error getting codec (%s)", err)
	}
	return &store{
		decodeKey:    keyer.NamespaceAndNameFromKey,
		codec:        codec,
		cl:           restCl,
		singularKind: ServiceBrokerKind,
		versioner:    etcd.APIObjectVersioner{},
		singularShell: func(ns, name string) runtime.Object {
			return &servicecatalog.Broker{
				TypeMeta: metav1.TypeMeta{
					Kind: ServiceBrokerKind.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      name,
				},
			}
		},
		listShell: func() runtime.Object {
			return &servicecatalog.BrokerList{}
		},
	}
}

func getInstanceTPRStorageIFace(
	t *testing.T,
	keyer Keyer,
	restCl *fake.RESTClient,
) *store {
	codec, err := testapi.GetCodecForObject(&sc.Instance{})
	if err != nil {
		t.Fatalf("error getting codec (%s)", err)
	}
	return &store{
		hasNamespace: true,
		decodeKey:    keyer.NamespaceAndNameFromKey,
		codec:        codec,
		cl:           restCl,
		singularKind: ServiceInstanceKind,
		versioner:    etcd.APIObjectVersioner{},
		singularShell: func(ns, name string) runtime.Object {
			return &servicecatalog.Instance{
				TypeMeta: metav1.TypeMeta{
					Kind: ServiceInstanceKind.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      name,
				},
			}
		},
		listShell: func() runtime.Object {
			return &servicecatalog.InstanceList{}
		},
	}
}

func verifyStorageError(err error, errorCode int) error {
	if err == nil {
		return errors.New("expected an error, but did not receive one")
	}
	storageErr, ok := err.(*storage.StorageError)
	if !ok {
		return fmt.Errorf(
			"expected a storage.StorageError, but got a %s",
			reflect.TypeOf(err),
		)
	}
	if storageErr.Code != errorCode {
		return fmt.Errorf(
			"expected error code %d, but got %d",
			errorCode,
			storageErr.Code,
		)
	}
	return nil
}

func deepCompare(
	obj1Name string,
	obj1 runtime.Object,
	obj2Name string,
	obj2 runtime.Object,
) error {
	if !equality.Semantic.DeepEqual(obj1, obj2) {
		return fmt.Errorf(
			"%s and %s are different: %s",
			obj1Name,
			obj2Name,
			// TODO: It's probably not an equivalent to semantic DeepEqual, is there semantic diff implementation?
			diff.ObjectReflectDiff(obj1, obj2),
		)
	}
	return nil
}
