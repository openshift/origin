/*
Copyright 2018 The Kubernetes Authors.

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

// ClusterServicePlansGetter has a method to return a ClusterServicePlanInterface.
// A group's client should implement this interface.
type ClusterServicePlansGetter interface {
	ClusterServicePlans() ClusterServicePlanInterface
}

// ClusterServicePlanInterface has methods to work with ClusterServicePlan resources.
type ClusterServicePlanInterface interface {
	Create(*servicecatalog.ClusterServicePlan) (*servicecatalog.ClusterServicePlan, error)
	Update(*servicecatalog.ClusterServicePlan) (*servicecatalog.ClusterServicePlan, error)
	UpdateStatus(*servicecatalog.ClusterServicePlan) (*servicecatalog.ClusterServicePlan, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*servicecatalog.ClusterServicePlan, error)
	List(opts v1.ListOptions) (*servicecatalog.ClusterServicePlanList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ClusterServicePlan, err error)
	ClusterServicePlanExpansion
}

// clusterServicePlans implements ClusterServicePlanInterface
type clusterServicePlans struct {
	client rest.Interface
}

// newClusterServicePlans returns a ClusterServicePlans
func newClusterServicePlans(c *ServicecatalogClient) *clusterServicePlans {
	return &clusterServicePlans{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterServicePlan, and returns the corresponding clusterServicePlan object, and an error if there is any.
func (c *clusterServicePlans) Get(name string, options v1.GetOptions) (result *servicecatalog.ClusterServicePlan, err error) {
	result = &servicecatalog.ClusterServicePlan{}
	err = c.client.Get().
		Resource("clusterserviceplans").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterServicePlans that match those selectors.
func (c *clusterServicePlans) List(opts v1.ListOptions) (result *servicecatalog.ClusterServicePlanList, err error) {
	result = &servicecatalog.ClusterServicePlanList{}
	err = c.client.Get().
		Resource("clusterserviceplans").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterServicePlans.
func (c *clusterServicePlans) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clusterserviceplans").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterServicePlan and creates it.  Returns the server's representation of the clusterServicePlan, and an error, if there is any.
func (c *clusterServicePlans) Create(clusterServicePlan *servicecatalog.ClusterServicePlan) (result *servicecatalog.ClusterServicePlan, err error) {
	result = &servicecatalog.ClusterServicePlan{}
	err = c.client.Post().
		Resource("clusterserviceplans").
		Body(clusterServicePlan).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterServicePlan and updates it. Returns the server's representation of the clusterServicePlan, and an error, if there is any.
func (c *clusterServicePlans) Update(clusterServicePlan *servicecatalog.ClusterServicePlan) (result *servicecatalog.ClusterServicePlan, err error) {
	result = &servicecatalog.ClusterServicePlan{}
	err = c.client.Put().
		Resource("clusterserviceplans").
		Name(clusterServicePlan.Name).
		Body(clusterServicePlan).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *clusterServicePlans) UpdateStatus(clusterServicePlan *servicecatalog.ClusterServicePlan) (result *servicecatalog.ClusterServicePlan, err error) {
	result = &servicecatalog.ClusterServicePlan{}
	err = c.client.Put().
		Resource("clusterserviceplans").
		Name(clusterServicePlan.Name).
		SubResource("status").
		Body(clusterServicePlan).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterServicePlan and deletes it. Returns an error if one occurs.
func (c *clusterServicePlans) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterserviceplans").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterServicePlans) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterserviceplans").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterServicePlan.
func (c *clusterServicePlans) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ClusterServicePlan, err error) {
	result = &servicecatalog.ClusterServicePlan{}
	err = c.client.Patch(pt).
		Resource("clusterserviceplans").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
