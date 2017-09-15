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

// FakeServiceInstances implements ServiceInstanceInterface
type FakeServiceInstances struct {
	Fake *FakeServicecatalogV1alpha1
	ns   string
}

var serviceinstancesResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "v1alpha1", Resource: "serviceinstances"}

var serviceinstancesKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "v1alpha1", Kind: "ServiceInstance"}

func (c *FakeServiceInstances) Create(serviceInstance *v1alpha1.ServiceInstance) (result *v1alpha1.ServiceInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(serviceinstancesResource, c.ns, serviceInstance), &v1alpha1.ServiceInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceInstance), err
}

func (c *FakeServiceInstances) Update(serviceInstance *v1alpha1.ServiceInstance) (result *v1alpha1.ServiceInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(serviceinstancesResource, c.ns, serviceInstance), &v1alpha1.ServiceInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceInstance), err
}

func (c *FakeServiceInstances) UpdateStatus(serviceInstance *v1alpha1.ServiceInstance) (*v1alpha1.ServiceInstance, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(serviceinstancesResource, "status", c.ns, serviceInstance), &v1alpha1.ServiceInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceInstance), err
}

func (c *FakeServiceInstances) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(serviceinstancesResource, c.ns, name), &v1alpha1.ServiceInstance{})

	return err
}

func (c *FakeServiceInstances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(serviceinstancesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ServiceInstanceList{})
	return err
}

func (c *FakeServiceInstances) Get(name string, options v1.GetOptions) (result *v1alpha1.ServiceInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(serviceinstancesResource, c.ns, name), &v1alpha1.ServiceInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceInstance), err
}

func (c *FakeServiceInstances) List(opts v1.ListOptions) (result *v1alpha1.ServiceInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(serviceinstancesResource, serviceinstancesKind, c.ns, opts), &v1alpha1.ServiceInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ServiceInstanceList{}
	for _, item := range obj.(*v1alpha1.ServiceInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested serviceInstances.
func (c *FakeServiceInstances) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(serviceinstancesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched serviceInstance.
func (c *FakeServiceInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ServiceInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(serviceinstancesResource, c.ns, name, data, subresources...), &v1alpha1.ServiceInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceInstance), err
}
