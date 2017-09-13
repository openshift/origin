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

// ServiceInstanceCredentialsGetter has a method to return a ServiceInstanceCredentialInterface.
// A group's client should implement this interface.
type ServiceInstanceCredentialsGetter interface {
	ServiceInstanceCredentials(namespace string) ServiceInstanceCredentialInterface
}

// ServiceInstanceCredentialInterface has methods to work with ServiceInstanceCredential resources.
type ServiceInstanceCredentialInterface interface {
	Create(*v1alpha1.ServiceInstanceCredential) (*v1alpha1.ServiceInstanceCredential, error)
	Update(*v1alpha1.ServiceInstanceCredential) (*v1alpha1.ServiceInstanceCredential, error)
	UpdateStatus(*v1alpha1.ServiceInstanceCredential) (*v1alpha1.ServiceInstanceCredential, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.ServiceInstanceCredential, error)
	List(opts v1.ListOptions) (*v1alpha1.ServiceInstanceCredentialList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ServiceInstanceCredential, err error)
	ServiceInstanceCredentialExpansion
}

// serviceInstanceCredentials implements ServiceInstanceCredentialInterface
type serviceInstanceCredentials struct {
	client rest.Interface
	ns     string
}

// newServiceInstanceCredentials returns a ServiceInstanceCredentials
func newServiceInstanceCredentials(c *ServicecatalogV1alpha1Client, namespace string) *serviceInstanceCredentials {
	return &serviceInstanceCredentials{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a serviceInstanceCredential and creates it.  Returns the server's representation of the serviceInstanceCredential, and an error, if there is any.
func (c *serviceInstanceCredentials) Create(serviceInstanceCredential *v1alpha1.ServiceInstanceCredential) (result *v1alpha1.ServiceInstanceCredential, err error) {
	result = &v1alpha1.ServiceInstanceCredential{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		Body(serviceInstanceCredential).
		Do().
		Into(result)
	return
}

// Update takes the representation of a serviceInstanceCredential and updates it. Returns the server's representation of the serviceInstanceCredential, and an error, if there is any.
func (c *serviceInstanceCredentials) Update(serviceInstanceCredential *v1alpha1.ServiceInstanceCredential) (result *v1alpha1.ServiceInstanceCredential, err error) {
	result = &v1alpha1.ServiceInstanceCredential{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		Name(serviceInstanceCredential.Name).
		Body(serviceInstanceCredential).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *serviceInstanceCredentials) UpdateStatus(serviceInstanceCredential *v1alpha1.ServiceInstanceCredential) (result *v1alpha1.ServiceInstanceCredential, err error) {
	result = &v1alpha1.ServiceInstanceCredential{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		Name(serviceInstanceCredential.Name).
		SubResource("status").
		Body(serviceInstanceCredential).
		Do().
		Into(result)
	return
}

// Delete takes name of the serviceInstanceCredential and deletes it. Returns an error if one occurs.
func (c *serviceInstanceCredentials) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *serviceInstanceCredentials) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the serviceInstanceCredential, and returns the corresponding serviceInstanceCredential object, and an error if there is any.
func (c *serviceInstanceCredentials) Get(name string, options v1.GetOptions) (result *v1alpha1.ServiceInstanceCredential, err error) {
	result = &v1alpha1.ServiceInstanceCredential{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ServiceInstanceCredentials that match those selectors.
func (c *serviceInstanceCredentials) List(opts v1.ListOptions) (result *v1alpha1.ServiceInstanceCredentialList, err error) {
	result = &v1alpha1.ServiceInstanceCredentialList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested serviceInstanceCredentials.
func (c *serviceInstanceCredentials) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched serviceInstanceCredential.
func (c *serviceInstanceCredentials) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ServiceInstanceCredential, err error) {
	result = &v1alpha1.ServiceInstanceCredential{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("serviceinstancecredentials").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
