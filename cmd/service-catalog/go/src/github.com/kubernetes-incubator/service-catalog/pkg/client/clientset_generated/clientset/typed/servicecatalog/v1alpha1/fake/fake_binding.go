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

// FakeBindings implements BindingInterface
type FakeBindings struct {
	Fake *FakeServicecatalogV1alpha1
	ns   string
}

var bindingsResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "v1alpha1", Resource: "bindings"}

func (c *FakeBindings) Create(binding *v1alpha1.Binding) (result *v1alpha1.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(bindingsResource, c.ns, binding), &v1alpha1.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Binding), err
}

func (c *FakeBindings) Update(binding *v1alpha1.Binding) (result *v1alpha1.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(bindingsResource, c.ns, binding), &v1alpha1.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Binding), err
}

func (c *FakeBindings) UpdateStatus(binding *v1alpha1.Binding) (*v1alpha1.Binding, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(bindingsResource, "status", c.ns, binding), &v1alpha1.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Binding), err
}

func (c *FakeBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(bindingsResource, c.ns, name), &v1alpha1.Binding{})

	return err
}

func (c *FakeBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(bindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.BindingList{})
	return err
}

func (c *FakeBindings) Get(name string, options v1.GetOptions) (result *v1alpha1.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(bindingsResource, c.ns, name), &v1alpha1.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Binding), err
}

func (c *FakeBindings) List(opts v1.ListOptions) (result *v1alpha1.BindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(bindingsResource, c.ns, opts), &v1alpha1.BindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.BindingList{}
	for _, item := range obj.(*v1alpha1.BindingList).Items {
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
func (c *FakeBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Binding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(bindingsResource, c.ns, name, data, subresources...), &v1alpha1.Binding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Binding), err
}
