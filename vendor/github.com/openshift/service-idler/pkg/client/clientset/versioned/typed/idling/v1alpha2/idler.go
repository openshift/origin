/*
Copyright 2018 Red Hat, Inc.

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

package v1alpha2

import (
	v1alpha2 "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	scheme "github.com/openshift/service-idler/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// IdlersGetter has a method to return a IdlerInterface.
// A group's client should implement this interface.
type IdlersGetter interface {
	Idlers(namespace string) IdlerInterface
}

// IdlerInterface has methods to work with Idler resources.
type IdlerInterface interface {
	Create(*v1alpha2.Idler) (*v1alpha2.Idler, error)
	Update(*v1alpha2.Idler) (*v1alpha2.Idler, error)
	UpdateStatus(*v1alpha2.Idler) (*v1alpha2.Idler, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha2.Idler, error)
	List(opts v1.ListOptions) (*v1alpha2.IdlerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha2.Idler, err error)
	IdlerExpansion
}

// idlers implements IdlerInterface
type idlers struct {
	client rest.Interface
	ns     string
}

// newIdlers returns a Idlers
func newIdlers(c *IdlingV1alpha2Client, namespace string) *idlers {
	return &idlers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the idler, and returns the corresponding idler object, and an error if there is any.
func (c *idlers) Get(name string, options v1.GetOptions) (result *v1alpha2.Idler, err error) {
	result = &v1alpha2.Idler{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("idlers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Idlers that match those selectors.
func (c *idlers) List(opts v1.ListOptions) (result *v1alpha2.IdlerList, err error) {
	result = &v1alpha2.IdlerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("idlers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested idlers.
func (c *idlers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("idlers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a idler and creates it.  Returns the server's representation of the idler, and an error, if there is any.
func (c *idlers) Create(idler *v1alpha2.Idler) (result *v1alpha2.Idler, err error) {
	result = &v1alpha2.Idler{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("idlers").
		Body(idler).
		Do().
		Into(result)
	return
}

// Update takes the representation of a idler and updates it. Returns the server's representation of the idler, and an error, if there is any.
func (c *idlers) Update(idler *v1alpha2.Idler) (result *v1alpha2.Idler, err error) {
	result = &v1alpha2.Idler{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("idlers").
		Name(idler.Name).
		Body(idler).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *idlers) UpdateStatus(idler *v1alpha2.Idler) (result *v1alpha2.Idler, err error) {
	result = &v1alpha2.Idler{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("idlers").
		Name(idler.Name).
		SubResource("status").
		Body(idler).
		Do().
		Into(result)
	return
}

// Delete takes name of the idler and deletes it. Returns an error if one occurs.
func (c *idlers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("idlers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *idlers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("idlers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched idler.
func (c *idlers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha2.Idler, err error) {
	result = &v1alpha2.Idler{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("idlers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
