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

package fake

import (
	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeServiceBrokers implements ServiceBrokerInterface
type FakeServiceBrokers struct {
	Fake *FakeServicecatalogV1alpha1
}

var servicebrokersResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "v1alpha1", Resource: "servicebrokers"}

var servicebrokersKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "v1alpha1", Kind: "ServiceBroker"}

func (c *FakeServiceBrokers) Create(serviceBroker *v1alpha1.ServiceBroker) (result *v1alpha1.ServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(servicebrokersResource, serviceBroker), &v1alpha1.ServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceBroker), err
}

func (c *FakeServiceBrokers) Update(serviceBroker *v1alpha1.ServiceBroker) (result *v1alpha1.ServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(servicebrokersResource, serviceBroker), &v1alpha1.ServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceBroker), err
}

func (c *FakeServiceBrokers) UpdateStatus(serviceBroker *v1alpha1.ServiceBroker) (*v1alpha1.ServiceBroker, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(servicebrokersResource, "status", serviceBroker), &v1alpha1.ServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceBroker), err
}

func (c *FakeServiceBrokers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(servicebrokersResource, name), &v1alpha1.ServiceBroker{})
	return err
}

func (c *FakeServiceBrokers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(servicebrokersResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ServiceBrokerList{})
	return err
}

func (c *FakeServiceBrokers) Get(name string, options v1.GetOptions) (result *v1alpha1.ServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(servicebrokersResource, name), &v1alpha1.ServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceBroker), err
}

func (c *FakeServiceBrokers) List(opts v1.ListOptions) (result *v1alpha1.ServiceBrokerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(servicebrokersResource, servicebrokersKind, opts), &v1alpha1.ServiceBrokerList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ServiceBrokerList{}
	for _, item := range obj.(*v1alpha1.ServiceBrokerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested serviceBrokers.
func (c *FakeServiceBrokers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(servicebrokersResource, opts))
}

// Patch applies the patch and returns the patched serviceBroker.
func (c *FakeServiceBrokers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(servicebrokersResource, name, data, subresources...), &v1alpha1.ServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceBroker), err
}
