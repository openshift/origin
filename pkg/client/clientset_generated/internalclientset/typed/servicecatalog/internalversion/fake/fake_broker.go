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
	servicecatalog "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBrokers implements BrokerInterface
type FakeBrokers struct {
	Fake *FakeServicecatalog
}

var brokersResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "", Resource: "brokers"}

func (c *FakeBrokers) Create(broker *servicecatalog.Broker) (result *servicecatalog.Broker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(brokersResource, broker), &servicecatalog.Broker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Broker), err
}

func (c *FakeBrokers) Update(broker *servicecatalog.Broker) (result *servicecatalog.Broker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(brokersResource, broker), &servicecatalog.Broker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Broker), err
}

func (c *FakeBrokers) UpdateStatus(broker *servicecatalog.Broker) (*servicecatalog.Broker, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(brokersResource, "status", broker), &servicecatalog.Broker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Broker), err
}

func (c *FakeBrokers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(brokersResource, name), &servicecatalog.Broker{})
	return err
}

func (c *FakeBrokers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(brokersResource, listOptions)

	_, err := c.Fake.Invokes(action, &servicecatalog.BrokerList{})
	return err
}

func (c *FakeBrokers) Get(name string, options v1.GetOptions) (result *servicecatalog.Broker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(brokersResource, name), &servicecatalog.Broker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Broker), err
}

func (c *FakeBrokers) List(opts v1.ListOptions) (result *servicecatalog.BrokerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(brokersResource, opts), &servicecatalog.BrokerList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &servicecatalog.BrokerList{}
	for _, item := range obj.(*servicecatalog.BrokerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested brokers.
func (c *FakeBrokers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(brokersResource, opts))
}

// Patch applies the patch and returns the patched broker.
func (c *FakeBrokers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.Broker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(brokersResource, name, data, subresources...), &servicecatalog.Broker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Broker), err
}
