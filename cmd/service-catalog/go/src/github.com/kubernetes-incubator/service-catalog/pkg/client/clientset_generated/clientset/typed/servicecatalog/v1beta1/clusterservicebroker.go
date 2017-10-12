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

// ClusterServiceBrokersGetter has a method to return a ClusterServiceBrokerInterface.
// A group's client should implement this interface.
type ClusterServiceBrokersGetter interface {
	ClusterServiceBrokers() ClusterServiceBrokerInterface
}

// ClusterServiceBrokerInterface has methods to work with ClusterServiceBroker resources.
type ClusterServiceBrokerInterface interface {
	Create(*v1beta1.ClusterServiceBroker) (*v1beta1.ClusterServiceBroker, error)
	Update(*v1beta1.ClusterServiceBroker) (*v1beta1.ClusterServiceBroker, error)
	UpdateStatus(*v1beta1.ClusterServiceBroker) (*v1beta1.ClusterServiceBroker, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.ClusterServiceBroker, error)
	List(opts v1.ListOptions) (*v1beta1.ClusterServiceBrokerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ClusterServiceBroker, err error)
	ClusterServiceBrokerExpansion
}

// clusterServiceBrokers implements ClusterServiceBrokerInterface
type clusterServiceBrokers struct {
	client rest.Interface
}

// newClusterServiceBrokers returns a ClusterServiceBrokers
func newClusterServiceBrokers(c *ServicecatalogV1beta1Client) *clusterServiceBrokers {
	return &clusterServiceBrokers{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterServiceBroker, and returns the corresponding clusterServiceBroker object, and an error if there is any.
func (c *clusterServiceBrokers) Get(name string, options v1.GetOptions) (result *v1beta1.ClusterServiceBroker, err error) {
	result = &v1beta1.ClusterServiceBroker{}
	err = c.client.Get().
		Resource("clusterservicebrokers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterServiceBrokers that match those selectors.
func (c *clusterServiceBrokers) List(opts v1.ListOptions) (result *v1beta1.ClusterServiceBrokerList, err error) {
	result = &v1beta1.ClusterServiceBrokerList{}
	err = c.client.Get().
		Resource("clusterservicebrokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterServiceBrokers.
func (c *clusterServiceBrokers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clusterservicebrokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterServiceBroker and creates it.  Returns the server's representation of the clusterServiceBroker, and an error, if there is any.
func (c *clusterServiceBrokers) Create(clusterServiceBroker *v1beta1.ClusterServiceBroker) (result *v1beta1.ClusterServiceBroker, err error) {
	result = &v1beta1.ClusterServiceBroker{}
	err = c.client.Post().
		Resource("clusterservicebrokers").
		Body(clusterServiceBroker).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterServiceBroker and updates it. Returns the server's representation of the clusterServiceBroker, and an error, if there is any.
func (c *clusterServiceBrokers) Update(clusterServiceBroker *v1beta1.ClusterServiceBroker) (result *v1beta1.ClusterServiceBroker, err error) {
	result = &v1beta1.ClusterServiceBroker{}
	err = c.client.Put().
		Resource("clusterservicebrokers").
		Name(clusterServiceBroker.Name).
		Body(clusterServiceBroker).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *clusterServiceBrokers) UpdateStatus(clusterServiceBroker *v1beta1.ClusterServiceBroker) (result *v1beta1.ClusterServiceBroker, err error) {
	result = &v1beta1.ClusterServiceBroker{}
	err = c.client.Put().
		Resource("clusterservicebrokers").
		Name(clusterServiceBroker.Name).
		SubResource("status").
		Body(clusterServiceBroker).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterServiceBroker and deletes it. Returns an error if one occurs.
func (c *clusterServiceBrokers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterservicebrokers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterServiceBrokers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterservicebrokers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterServiceBroker.
func (c *clusterServiceBrokers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ClusterServiceBroker, err error) {
	result = &v1beta1.ClusterServiceBroker{}
	err = c.client.Patch(pt).
		Resource("clusterservicebrokers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
