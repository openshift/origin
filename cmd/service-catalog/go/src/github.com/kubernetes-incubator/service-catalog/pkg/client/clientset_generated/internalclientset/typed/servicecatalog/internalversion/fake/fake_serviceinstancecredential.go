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

// FakeServiceInstanceCredentials implements ServiceInstanceCredentialInterface
type FakeServiceInstanceCredentials struct {
	Fake *FakeServicecatalog
	ns   string
}

var serviceinstancecredentialsResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "", Resource: "serviceinstancecredentials"}

var serviceinstancecredentialsKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "", Kind: "ServiceInstanceCredential"}

func (c *FakeServiceInstanceCredentials) Create(serviceInstanceCredential *servicecatalog.ServiceInstanceCredential) (result *servicecatalog.ServiceInstanceCredential, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(serviceinstancecredentialsResource, c.ns, serviceInstanceCredential), &servicecatalog.ServiceInstanceCredential{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceInstanceCredential), err
}

func (c *FakeServiceInstanceCredentials) Update(serviceInstanceCredential *servicecatalog.ServiceInstanceCredential) (result *servicecatalog.ServiceInstanceCredential, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(serviceinstancecredentialsResource, c.ns, serviceInstanceCredential), &servicecatalog.ServiceInstanceCredential{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceInstanceCredential), err
}

func (c *FakeServiceInstanceCredentials) UpdateStatus(serviceInstanceCredential *servicecatalog.ServiceInstanceCredential) (*servicecatalog.ServiceInstanceCredential, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(serviceinstancecredentialsResource, "status", c.ns, serviceInstanceCredential), &servicecatalog.ServiceInstanceCredential{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceInstanceCredential), err
}

func (c *FakeServiceInstanceCredentials) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(serviceinstancecredentialsResource, c.ns, name), &servicecatalog.ServiceInstanceCredential{})

	return err
}

func (c *FakeServiceInstanceCredentials) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(serviceinstancecredentialsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &servicecatalog.ServiceInstanceCredentialList{})
	return err
}

func (c *FakeServiceInstanceCredentials) Get(name string, options v1.GetOptions) (result *servicecatalog.ServiceInstanceCredential, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(serviceinstancecredentialsResource, c.ns, name), &servicecatalog.ServiceInstanceCredential{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceInstanceCredential), err
}

func (c *FakeServiceInstanceCredentials) List(opts v1.ListOptions) (result *servicecatalog.ServiceInstanceCredentialList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(serviceinstancecredentialsResource, serviceinstancecredentialsKind, c.ns, opts), &servicecatalog.ServiceInstanceCredentialList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &servicecatalog.ServiceInstanceCredentialList{}
	for _, item := range obj.(*servicecatalog.ServiceInstanceCredentialList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested serviceInstanceCredentials.
func (c *FakeServiceInstanceCredentials) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(serviceinstancecredentialsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched serviceInstanceCredential.
func (c *FakeServiceInstanceCredentials) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServiceInstanceCredential, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(serviceinstancecredentialsResource, c.ns, name, data, subresources...), &servicecatalog.ServiceInstanceCredential{})

	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceInstanceCredential), err
}
