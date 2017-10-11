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

// ClusterServiceClassesGetter has a method to return a ClusterServiceClassInterface.
// A group's client should implement this interface.
type ClusterServiceClassesGetter interface {
	ClusterServiceClasses() ClusterServiceClassInterface
}

// ClusterServiceClassInterface has methods to work with ClusterServiceClass resources.
type ClusterServiceClassInterface interface {
	Create(*v1beta1.ClusterServiceClass) (*v1beta1.ClusterServiceClass, error)
	Update(*v1beta1.ClusterServiceClass) (*v1beta1.ClusterServiceClass, error)
	UpdateStatus(*v1beta1.ClusterServiceClass) (*v1beta1.ClusterServiceClass, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.ClusterServiceClass, error)
	List(opts v1.ListOptions) (*v1beta1.ClusterServiceClassList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ClusterServiceClass, err error)
	ClusterServiceClassExpansion
}

// clusterServiceClasses implements ClusterServiceClassInterface
type clusterServiceClasses struct {
	client rest.Interface
}

// newClusterServiceClasses returns a ClusterServiceClasses
func newClusterServiceClasses(c *ServicecatalogV1beta1Client) *clusterServiceClasses {
	return &clusterServiceClasses{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterServiceClass, and returns the corresponding clusterServiceClass object, and an error if there is any.
func (c *clusterServiceClasses) Get(name string, options v1.GetOptions) (result *v1beta1.ClusterServiceClass, err error) {
	result = &v1beta1.ClusterServiceClass{}
	err = c.client.Get().
		Resource("clusterserviceclasses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterServiceClasses that match those selectors.
func (c *clusterServiceClasses) List(opts v1.ListOptions) (result *v1beta1.ClusterServiceClassList, err error) {
	result = &v1beta1.ClusterServiceClassList{}
	err = c.client.Get().
		Resource("clusterserviceclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterServiceClasses.
func (c *clusterServiceClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clusterserviceclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterServiceClass and creates it.  Returns the server's representation of the clusterServiceClass, and an error, if there is any.
func (c *clusterServiceClasses) Create(clusterServiceClass *v1beta1.ClusterServiceClass) (result *v1beta1.ClusterServiceClass, err error) {
	result = &v1beta1.ClusterServiceClass{}
	err = c.client.Post().
		Resource("clusterserviceclasses").
		Body(clusterServiceClass).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterServiceClass and updates it. Returns the server's representation of the clusterServiceClass, and an error, if there is any.
func (c *clusterServiceClasses) Update(clusterServiceClass *v1beta1.ClusterServiceClass) (result *v1beta1.ClusterServiceClass, err error) {
	result = &v1beta1.ClusterServiceClass{}
	err = c.client.Put().
		Resource("clusterserviceclasses").
		Name(clusterServiceClass.Name).
		Body(clusterServiceClass).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *clusterServiceClasses) UpdateStatus(clusterServiceClass *v1beta1.ClusterServiceClass) (result *v1beta1.ClusterServiceClass, err error) {
	result = &v1beta1.ClusterServiceClass{}
	err = c.client.Put().
		Resource("clusterserviceclasses").
		Name(clusterServiceClass.Name).
		SubResource("status").
		Body(clusterServiceClass).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterServiceClass and deletes it. Returns an error if one occurs.
func (c *clusterServiceClasses) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterserviceclasses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterServiceClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterserviceclasses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterServiceClass.
func (c *clusterServiceClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ClusterServiceClass, err error) {
	result = &v1beta1.ClusterServiceClass{}
	err = c.client.Patch(pt).
		Resource("clusterserviceclasses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
