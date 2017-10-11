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

// ServiceBindingsGetter has a method to return a ServiceBindingInterface.
// A group's client should implement this interface.
type ServiceBindingsGetter interface {
	ServiceBindings(namespace string) ServiceBindingInterface
}

// ServiceBindingInterface has methods to work with ServiceBinding resources.
type ServiceBindingInterface interface {
	Create(*servicecatalog.ServiceBinding) (*servicecatalog.ServiceBinding, error)
	Update(*servicecatalog.ServiceBinding) (*servicecatalog.ServiceBinding, error)
	UpdateStatus(*servicecatalog.ServiceBinding) (*servicecatalog.ServiceBinding, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*servicecatalog.ServiceBinding, error)
	List(opts v1.ListOptions) (*servicecatalog.ServiceBindingList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServiceBinding, err error)
	ServiceBindingExpansion
}

// serviceBindings implements ServiceBindingInterface
type serviceBindings struct {
	client rest.Interface
	ns     string
}

// newServiceBindings returns a ServiceBindings
func newServiceBindings(c *ServicecatalogClient, namespace string) *serviceBindings {
	return &serviceBindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the serviceBinding, and returns the corresponding serviceBinding object, and an error if there is any.
func (c *serviceBindings) Get(name string, options v1.GetOptions) (result *servicecatalog.ServiceBinding, err error) {
	result = &servicecatalog.ServiceBinding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("servicebindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ServiceBindings that match those selectors.
func (c *serviceBindings) List(opts v1.ListOptions) (result *servicecatalog.ServiceBindingList, err error) {
	result = &servicecatalog.ServiceBindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("servicebindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested serviceBindings.
func (c *serviceBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("servicebindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a serviceBinding and creates it.  Returns the server's representation of the serviceBinding, and an error, if there is any.
func (c *serviceBindings) Create(serviceBinding *servicecatalog.ServiceBinding) (result *servicecatalog.ServiceBinding, err error) {
	result = &servicecatalog.ServiceBinding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("servicebindings").
		Body(serviceBinding).
		Do().
		Into(result)
	return
}

// Update takes the representation of a serviceBinding and updates it. Returns the server's representation of the serviceBinding, and an error, if there is any.
func (c *serviceBindings) Update(serviceBinding *servicecatalog.ServiceBinding) (result *servicecatalog.ServiceBinding, err error) {
	result = &servicecatalog.ServiceBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("servicebindings").
		Name(serviceBinding.Name).
		Body(serviceBinding).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *serviceBindings) UpdateStatus(serviceBinding *servicecatalog.ServiceBinding) (result *servicecatalog.ServiceBinding, err error) {
	result = &servicecatalog.ServiceBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("servicebindings").
		Name(serviceBinding.Name).
		SubResource("status").
		Body(serviceBinding).
		Do().
		Into(result)
	return
}

// Delete takes name of the serviceBinding and deletes it. Returns an error if one occurs.
func (c *serviceBindings) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("servicebindings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *serviceBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("servicebindings").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched serviceBinding.
func (c *serviceBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServiceBinding, err error) {
	result = &servicecatalog.ServiceBinding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("servicebindings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
