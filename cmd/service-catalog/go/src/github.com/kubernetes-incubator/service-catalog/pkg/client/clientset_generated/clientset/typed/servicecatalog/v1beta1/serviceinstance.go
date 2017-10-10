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

package v1beta1

import (
	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scheme "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ServiceInstancesGetter has a method to return a ServiceInstanceInterface.
// A group's client should implement this interface.
type ServiceInstancesGetter interface {
	ServiceInstances(namespace string) ServiceInstanceInterface
}

// ServiceInstanceInterface has methods to work with ServiceInstance resources.
type ServiceInstanceInterface interface {
	Create(*v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error)
	Update(*v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error)
	UpdateStatus(*v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.ServiceInstance, error)
	List(opts v1.ListOptions) (*v1beta1.ServiceInstanceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ServiceInstance, err error)
	ServiceInstanceExpansion
}

// serviceInstances implements ServiceInstanceInterface
type serviceInstances struct {
	client rest.Interface
	ns     string
}

// newServiceInstances returns a ServiceInstances
func newServiceInstances(c *ServicecatalogV1beta1Client, namespace string) *serviceInstances {
	return &serviceInstances{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the serviceInstance, and returns the corresponding serviceInstance object, and an error if there is any.
func (c *serviceInstances) Get(name string, options v1.GetOptions) (result *v1beta1.ServiceInstance, err error) {
	result = &v1beta1.ServiceInstance{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("serviceinstances").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ServiceInstances that match those selectors.
func (c *serviceInstances) List(opts v1.ListOptions) (result *v1beta1.ServiceInstanceList, err error) {
	result = &v1beta1.ServiceInstanceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("serviceinstances").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested serviceInstances.
func (c *serviceInstances) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("serviceinstances").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a serviceInstance and creates it.  Returns the server's representation of the serviceInstance, and an error, if there is any.
func (c *serviceInstances) Create(serviceInstance *v1beta1.ServiceInstance) (result *v1beta1.ServiceInstance, err error) {
	result = &v1beta1.ServiceInstance{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("serviceinstances").
		Body(serviceInstance).
		Do().
		Into(result)
	return
}

// Update takes the representation of a serviceInstance and updates it. Returns the server's representation of the serviceInstance, and an error, if there is any.
func (c *serviceInstances) Update(serviceInstance *v1beta1.ServiceInstance) (result *v1beta1.ServiceInstance, err error) {
	result = &v1beta1.ServiceInstance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("serviceinstances").
		Name(serviceInstance.Name).
		Body(serviceInstance).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *serviceInstances) UpdateStatus(serviceInstance *v1beta1.ServiceInstance) (result *v1beta1.ServiceInstance, err error) {
	result = &v1beta1.ServiceInstance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("serviceinstances").
		Name(serviceInstance.Name).
		SubResource("status").
		Body(serviceInstance).
		Do().
		Into(result)
	return
}

// Delete takes name of the serviceInstance and deletes it. Returns an error if one occurs.
func (c *serviceInstances) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("serviceinstances").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *serviceInstances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("serviceinstances").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched serviceInstance.
func (c *serviceInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ServiceInstance, err error) {
	result = &v1beta1.ServiceInstance{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("serviceinstances").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
