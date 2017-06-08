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

// BrokersGetter has a method to return a BrokerInterface.
// A group's client should implement this interface.
type BrokersGetter interface {
	Brokers() BrokerInterface
}

// BrokerInterface has methods to work with Broker resources.
type BrokerInterface interface {
	Create(*servicecatalog.Broker) (*servicecatalog.Broker, error)
	Update(*servicecatalog.Broker) (*servicecatalog.Broker, error)
	UpdateStatus(*servicecatalog.Broker) (*servicecatalog.Broker, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*servicecatalog.Broker, error)
	List(opts v1.ListOptions) (*servicecatalog.BrokerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.Broker, err error)
	BrokerExpansion
}

// brokers implements BrokerInterface
type brokers struct {
	client rest.Interface
}

// newBrokers returns a Brokers
func newBrokers(c *ServicecatalogClient) *brokers {
	return &brokers{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a broker and creates it.  Returns the server's representation of the broker, and an error, if there is any.
func (c *brokers) Create(broker *servicecatalog.Broker) (result *servicecatalog.Broker, err error) {
	result = &servicecatalog.Broker{}
	err = c.client.Post().
		Resource("brokers").
		Body(broker).
		Do().
		Into(result)
	return
}

// Update takes the representation of a broker and updates it. Returns the server's representation of the broker, and an error, if there is any.
func (c *brokers) Update(broker *servicecatalog.Broker) (result *servicecatalog.Broker, err error) {
	result = &servicecatalog.Broker{}
	err = c.client.Put().
		Resource("brokers").
		Name(broker.Name).
		Body(broker).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *brokers) UpdateStatus(broker *servicecatalog.Broker) (result *servicecatalog.Broker, err error) {
	result = &servicecatalog.Broker{}
	err = c.client.Put().
		Resource("brokers").
		Name(broker.Name).
		SubResource("status").
		Body(broker).
		Do().
		Into(result)
	return
}

// Delete takes name of the broker and deletes it. Returns an error if one occurs.
func (c *brokers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("brokers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *brokers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("brokers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the broker, and returns the corresponding broker object, and an error if there is any.
func (c *brokers) Get(name string, options v1.GetOptions) (result *servicecatalog.Broker, err error) {
	result = &servicecatalog.Broker{}
	err = c.client.Get().
		Resource("brokers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Brokers that match those selectors.
func (c *brokers) List(opts v1.ListOptions) (result *servicecatalog.BrokerList, err error) {
	result = &servicecatalog.BrokerList{}
	err = c.client.Get().
		Resource("brokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested brokers.
func (c *brokers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("brokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched broker.
func (c *brokers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.Broker, err error) {
	result = &servicecatalog.Broker{}
	err = c.client.Patch(pt).
		Resource("brokers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
