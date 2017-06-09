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

// InstancesGetter has a method to return a InstanceInterface.
// A group's client should implement this interface.
type InstancesGetter interface {
	Instances(namespace string) InstanceInterface
}

// InstanceInterface has methods to work with Instance resources.
type InstanceInterface interface {
	Create(*servicecatalog.Instance) (*servicecatalog.Instance, error)
	Update(*servicecatalog.Instance) (*servicecatalog.Instance, error)
	UpdateStatus(*servicecatalog.Instance) (*servicecatalog.Instance, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*servicecatalog.Instance, error)
	List(opts v1.ListOptions) (*servicecatalog.InstanceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.Instance, err error)
	InstanceExpansion
}

// instances implements InstanceInterface
type instances struct {
	client rest.Interface
	ns     string
}

// newInstances returns a Instances
func newInstances(c *ServicecatalogClient, namespace string) *instances {
	return &instances{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a instance and creates it.  Returns the server's representation of the instance, and an error, if there is any.
func (c *instances) Create(instance *servicecatalog.Instance) (result *servicecatalog.Instance, err error) {
	result = &servicecatalog.Instance{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("instances").
		Body(instance).
		Do().
		Into(result)
	return
}

// Update takes the representation of a instance and updates it. Returns the server's representation of the instance, and an error, if there is any.
func (c *instances) Update(instance *servicecatalog.Instance) (result *servicecatalog.Instance, err error) {
	result = &servicecatalog.Instance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("instances").
		Name(instance.Name).
		Body(instance).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *instances) UpdateStatus(instance *servicecatalog.Instance) (result *servicecatalog.Instance, err error) {
	result = &servicecatalog.Instance{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("instances").
		Name(instance.Name).
		SubResource("status").
		Body(instance).
		Do().
		Into(result)
	return
}

// Delete takes name of the instance and deletes it. Returns an error if one occurs.
func (c *instances) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("instances").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *instances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("instances").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the instance, and returns the corresponding instance object, and an error if there is any.
func (c *instances) Get(name string, options v1.GetOptions) (result *servicecatalog.Instance, err error) {
	result = &servicecatalog.Instance{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("instances").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Instances that match those selectors.
func (c *instances) List(opts v1.ListOptions) (result *servicecatalog.InstanceList, err error) {
	result = &servicecatalog.InstanceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("instances").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested instances.
func (c *instances) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("instances").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched instance.
func (c *instances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.Instance, err error) {
	result = &servicecatalog.Instance{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("instances").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
