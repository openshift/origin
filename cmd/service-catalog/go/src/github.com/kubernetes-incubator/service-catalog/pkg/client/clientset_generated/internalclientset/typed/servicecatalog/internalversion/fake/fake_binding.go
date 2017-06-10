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

// FakeBindings implements BindingInterface
type FakeBindings struct {
	Fake *FakeServicecatalog
	ns   string
}

var bindingsResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "", Resource: "bindings"}

func (c *FakeBindings) Create(binding *servicecatalog.Binding) (result *servicecatalog.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(bindingsResource, c.ns, binding), &servicecatalog.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Binding), err
}

func (c *FakeBindings) Update(binding *servicecatalog.Binding) (result *servicecatalog.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(bindingsResource, c.ns, binding), &servicecatalog.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Binding), err
}

func (c *FakeBindings) UpdateStatus(binding *servicecatalog.Binding) (*servicecatalog.Binding, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(bindingsResource, "status", c.ns, binding), &servicecatalog.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Binding), err
}

func (c *FakeBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(bindingsResource, c.ns, name), &servicecatalog.Binding{})

	return err
}

func (c *FakeBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(bindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &servicecatalog.BindingList{})
	return err
}

func (c *FakeBindings) Get(name string, options v1.GetOptions) (result *servicecatalog.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(bindingsResource, c.ns, name), &servicecatalog.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Binding), err
}

func (c *FakeBindings) List(opts v1.ListOptions) (result *servicecatalog.BindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(bindingsResource, c.ns, opts), &servicecatalog.BindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &servicecatalog.BindingList{}
	for _, item := range obj.(*servicecatalog.BindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested bindings.
func (c *FakeBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(bindingsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched binding.
func (c *FakeBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(bindingsResource, c.ns, name, data, subresources...), &servicecatalog.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.Binding), err
}
