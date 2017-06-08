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

package v1alpha1

import (
	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	scheme "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BindingsGetter has a method to return a BindingInterface.
// A group's client should implement this interface.
type BindingsGetter interface {
	Bindings(namespace string) BindingInterface
}

// BindingInterface has methods to work with Binding resources.
type BindingInterface interface {
	Create(*v1alpha1.Binding) (*v1alpha1.Binding, error)
	Update(*v1alpha1.Binding) (*v1alpha1.Binding, error)
	UpdateStatus(*v1alpha1.Binding) (*v1alpha1.Binding, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.Binding, error)
	List(opts v1.ListOptions) (*v1alpha1.BindingList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Binding, err error)
	BindingExpansion
}

// bindings implements BindingInterface
type bindings struct {
	client rest.Interface
	ns     string
}

// newBindings returns a Bindings
func newBindings(c *ServicecatalogV1alpha1Client, namespace string) *bindings {
	return &bindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a binding and creates it.  Returns the server's representation of the binding, and an error, if there is any.
func (c *bindings) Create(binding *v1alpha1.Binding) (result *v1alpha1.Binding, err error) {
	result = &v1alpha1.Binding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("bindings").
		Body(binding).
		Do().
		Into(result)
	return
}

// Update takes the representation of a binding and updates it. Returns the server's representation of the binding, and an error, if there is any.
func (c *bindings) Update(binding *v1alpha1.Binding) (result *v1alpha1.Binding, err error) {
	result = &v1alpha1.Binding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("bindings").
		Name(binding.Name).
		Body(binding).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *bindings) UpdateStatus(binding *v1alpha1.Binding) (result *v1alpha1.Binding, err error) {
	result = &v1alpha1.Binding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("bindings").
		Name(binding.Name).
		SubResource("status").
		Body(binding).
		Do().
		Into(result)
	return
}

// Delete takes name of the binding and deletes it. Returns an error if one occurs.
func (c *bindings) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("bindings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *bindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the binding, and returns the corresponding binding object, and an error if there is any.
func (c *bindings) Get(name string, options v1.GetOptions) (result *v1alpha1.Binding, err error) {
	result = &v1alpha1.Binding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("bindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Bindings that match those selectors.
func (c *bindings) List(opts v1.ListOptions) (result *v1alpha1.BindingList, err error) {
	result = &v1alpha1.BindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested bindings.
func (c *bindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched binding.
func (c *bindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Binding, err error) {
	result = &v1alpha1.Binding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("bindings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
