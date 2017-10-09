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

// ServicePlansGetter has a method to return a ServicePlanInterface.
// A group's client should implement this interface.
type ServicePlansGetter interface {
	ServicePlans() ServicePlanInterface
}

// ServicePlanInterface has methods to work with ServicePlan resources.
type ServicePlanInterface interface {
	Create(*servicecatalog.ServicePlan) (*servicecatalog.ServicePlan, error)
	Update(*servicecatalog.ServicePlan) (*servicecatalog.ServicePlan, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*servicecatalog.ServicePlan, error)
	List(opts v1.ListOptions) (*servicecatalog.ServicePlanList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServicePlan, err error)
	ServicePlanExpansion
}

// servicePlans implements ServicePlanInterface
type servicePlans struct {
	client rest.Interface
}

// newServicePlans returns a ServicePlans
func newServicePlans(c *ServicecatalogClient) *servicePlans {
	return &servicePlans{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a servicePlan and creates it.  Returns the server's representation of the servicePlan, and an error, if there is any.
func (c *servicePlans) Create(servicePlan *servicecatalog.ServicePlan) (result *servicecatalog.ServicePlan, err error) {
	result = &servicecatalog.ServicePlan{}
	err = c.client.Post().
		Resource("serviceplans").
		Body(servicePlan).
		Do().
		Into(result)
	return
}

// Update takes the representation of a servicePlan and updates it. Returns the server's representation of the servicePlan, and an error, if there is any.
func (c *servicePlans) Update(servicePlan *servicecatalog.ServicePlan) (result *servicecatalog.ServicePlan, err error) {
	result = &servicecatalog.ServicePlan{}
	err = c.client.Put().
		Resource("serviceplans").
		Name(servicePlan.Name).
		Body(servicePlan).
		Do().
		Into(result)
	return
}

// Delete takes name of the servicePlan and deletes it. Returns an error if one occurs.
func (c *servicePlans) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("serviceplans").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *servicePlans) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("serviceplans").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the servicePlan, and returns the corresponding servicePlan object, and an error if there is any.
func (c *servicePlans) Get(name string, options v1.GetOptions) (result *servicecatalog.ServicePlan, err error) {
	result = &servicecatalog.ServicePlan{}
	err = c.client.Get().
		Resource("serviceplans").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ServicePlans that match those selectors.
func (c *servicePlans) List(opts v1.ListOptions) (result *servicecatalog.ServicePlanList, err error) {
	result = &servicecatalog.ServicePlanList{}
	err = c.client.Get().
		Resource("serviceplans").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested servicePlans.
func (c *servicePlans) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("serviceplans").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched servicePlan.
func (c *servicePlans) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServicePlan, err error) {
	result = &servicecatalog.ServicePlan{}
	err = c.client.Patch(pt).
		Resource("serviceplans").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
