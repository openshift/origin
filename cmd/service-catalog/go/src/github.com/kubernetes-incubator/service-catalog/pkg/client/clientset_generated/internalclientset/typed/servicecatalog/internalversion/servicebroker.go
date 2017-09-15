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

package internalversion

import (
	servicecatalog "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scheme "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ServiceBrokersGetter has a method to return a ServiceBrokerInterface.
// A group's client should implement this interface.
type ServiceBrokersGetter interface {
	ServiceBrokers() ServiceBrokerInterface
}

// ServiceBrokerInterface has methods to work with ServiceBroker resources.
type ServiceBrokerInterface interface {
	Create(*servicecatalog.ServiceBroker) (*servicecatalog.ServiceBroker, error)
	Update(*servicecatalog.ServiceBroker) (*servicecatalog.ServiceBroker, error)
	UpdateStatus(*servicecatalog.ServiceBroker) (*servicecatalog.ServiceBroker, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*servicecatalog.ServiceBroker, error)
	List(opts v1.ListOptions) (*servicecatalog.ServiceBrokerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServiceBroker, err error)
	ServiceBrokerExpansion
}

// serviceBrokers implements ServiceBrokerInterface
type serviceBrokers struct {
	client rest.Interface
}

// newServiceBrokers returns a ServiceBrokers
func newServiceBrokers(c *ServicecatalogClient) *serviceBrokers {
	return &serviceBrokers{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a serviceBroker and creates it.  Returns the server's representation of the serviceBroker, and an error, if there is any.
func (c *serviceBrokers) Create(serviceBroker *servicecatalog.ServiceBroker) (result *servicecatalog.ServiceBroker, err error) {
	result = &servicecatalog.ServiceBroker{}
	err = c.client.Post().
		Resource("servicebrokers").
		Body(serviceBroker).
		Do().
		Into(result)
	return
}

// Update takes the representation of a serviceBroker and updates it. Returns the server's representation of the serviceBroker, and an error, if there is any.
func (c *serviceBrokers) Update(serviceBroker *servicecatalog.ServiceBroker) (result *servicecatalog.ServiceBroker, err error) {
	result = &servicecatalog.ServiceBroker{}
	err = c.client.Put().
		Resource("servicebrokers").
		Name(serviceBroker.Name).
		Body(serviceBroker).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *serviceBrokers) UpdateStatus(serviceBroker *servicecatalog.ServiceBroker) (result *servicecatalog.ServiceBroker, err error) {
	result = &servicecatalog.ServiceBroker{}
	err = c.client.Put().
		Resource("servicebrokers").
		Name(serviceBroker.Name).
		SubResource("status").
		Body(serviceBroker).
		Do().
		Into(result)
	return
}

// Delete takes name of the serviceBroker and deletes it. Returns an error if one occurs.
func (c *serviceBrokers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("servicebrokers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *serviceBrokers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("servicebrokers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the serviceBroker, and returns the corresponding serviceBroker object, and an error if there is any.
func (c *serviceBrokers) Get(name string, options v1.GetOptions) (result *servicecatalog.ServiceBroker, err error) {
	result = &servicecatalog.ServiceBroker{}
	err = c.client.Get().
		Resource("servicebrokers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ServiceBrokers that match those selectors.
func (c *serviceBrokers) List(opts v1.ListOptions) (result *servicecatalog.ServiceBrokerList, err error) {
	result = &servicecatalog.ServiceBrokerList{}
	err = c.client.Get().
		Resource("servicebrokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested serviceBrokers.
func (c *serviceBrokers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("servicebrokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched serviceBroker.
func (c *serviceBrokers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServiceBroker, err error) {
	result = &servicecatalog.ServiceBroker{}
	err = c.client.Patch(pt).
		Resource("servicebrokers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
