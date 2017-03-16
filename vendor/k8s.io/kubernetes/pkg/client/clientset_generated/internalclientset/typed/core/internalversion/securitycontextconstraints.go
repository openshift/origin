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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	api "k8s.io/kubernetes/pkg/api"
	scheme "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/scheme"
)

// SecurityContextConstraintsGetter has a method to return a SecurityContextConstraintsInterface.
// A group's client should implement this interface.
type SecurityContextConstraintsGetter interface {
	SecurityContextConstraints() SecurityContextConstraintsInterface
}

// SecurityContextConstraintsInterface has methods to work with SecurityContextConstraints resources.
type SecurityContextConstraintsInterface interface {
	Create(*api.SecurityContextConstraints) (*api.SecurityContextConstraints, error)
	Update(*api.SecurityContextConstraints) (*api.SecurityContextConstraints, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*api.SecurityContextConstraints, error)
	List(opts v1.ListOptions) (*api.SecurityContextConstraintsList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.SecurityContextConstraints, err error)
	SecurityContextConstraintsExpansion
}

// securityContextConstraints implements SecurityContextConstraintsInterface
type securityContextConstraints struct {
	client rest.Interface
}

// newSecurityContextConstraints returns a SecurityContextConstraints
func newSecurityContextConstraints(c *CoreClient) *securityContextConstraints {
	return &securityContextConstraints{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a securityContextConstraints and creates it.  Returns the server's representation of the securityContextConstraints, and an error, if there is any.
func (c *securityContextConstraints) Create(securityContextConstraints *api.SecurityContextConstraints) (result *api.SecurityContextConstraints, err error) {
	result = &api.SecurityContextConstraints{}
	err = c.client.Post().
		Resource("securitycontextconstraints").
		Body(securityContextConstraints).
		Do().
		Into(result)
	return
}

// Update takes the representation of a securityContextConstraints and updates it. Returns the server's representation of the securityContextConstraints, and an error, if there is any.
func (c *securityContextConstraints) Update(securityContextConstraints *api.SecurityContextConstraints) (result *api.SecurityContextConstraints, err error) {
	result = &api.SecurityContextConstraints{}
	err = c.client.Put().
		Resource("securitycontextconstraints").
		Name(securityContextConstraints.Name).
		Body(securityContextConstraints).
		Do().
		Into(result)
	return
}

// Delete takes name of the securityContextConstraints and deletes it. Returns an error if one occurs.
func (c *securityContextConstraints) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("securitycontextconstraints").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *securityContextConstraints) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("securitycontextconstraints").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the securityContextConstraints, and returns the corresponding securityContextConstraints object, and an error if there is any.
func (c *securityContextConstraints) Get(name string, options v1.GetOptions) (result *api.SecurityContextConstraints, err error) {
	result = &api.SecurityContextConstraints{}
	err = c.client.Get().
		Resource("securitycontextconstraints").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SecurityContextConstraints that match those selectors.
func (c *securityContextConstraints) List(opts v1.ListOptions) (result *api.SecurityContextConstraintsList, err error) {
	result = &api.SecurityContextConstraintsList{}
	err = c.client.Get().
		Resource("securitycontextconstraints").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested securityContextConstraints.
func (c *securityContextConstraints) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("securitycontextconstraints").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched securityContextConstraints.
func (c *securityContextConstraints) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.SecurityContextConstraints, err error) {
	result = &api.SecurityContextConstraints{}
	err = c.client.Patch(pt).
		Resource("securitycontextconstraints").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
