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

package fake

import (
	servicecatalog "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterServicePlans implements ClusterServicePlanInterface
type FakeClusterServicePlans struct {
	Fake *FakeServicecatalog
}

var clusterserviceplansResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "", Resource: "clusterserviceplans"}

var clusterserviceplansKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "", Kind: "ClusterServicePlan"}

// Get takes name of the clusterServicePlan, and returns the corresponding clusterServicePlan object, and an error if there is any.
func (c *FakeClusterServicePlans) Get(name string, options v1.GetOptions) (result *servicecatalog.ClusterServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterserviceplansResource, name), &servicecatalog.ClusterServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ClusterServicePlan), err
}

// List takes label and field selectors, and returns the list of ClusterServicePlans that match those selectors.
func (c *FakeClusterServicePlans) List(opts v1.ListOptions) (result *servicecatalog.ClusterServicePlanList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterserviceplansResource, clusterserviceplansKind, opts), &servicecatalog.ClusterServicePlanList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &servicecatalog.ClusterServicePlanList{}
	for _, item := range obj.(*servicecatalog.ClusterServicePlanList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterServicePlans.
func (c *FakeClusterServicePlans) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterserviceplansResource, opts))
}

// Create takes the representation of a clusterServicePlan and creates it.  Returns the server's representation of the clusterServicePlan, and an error, if there is any.
func (c *FakeClusterServicePlans) Create(clusterServicePlan *servicecatalog.ClusterServicePlan) (result *servicecatalog.ClusterServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterserviceplansResource, clusterServicePlan), &servicecatalog.ClusterServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ClusterServicePlan), err
}

// Update takes the representation of a clusterServicePlan and updates it. Returns the server's representation of the clusterServicePlan, and an error, if there is any.
func (c *FakeClusterServicePlans) Update(clusterServicePlan *servicecatalog.ClusterServicePlan) (result *servicecatalog.ClusterServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterserviceplansResource, clusterServicePlan), &servicecatalog.ClusterServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ClusterServicePlan), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterServicePlans) UpdateStatus(clusterServicePlan *servicecatalog.ClusterServicePlan) (*servicecatalog.ClusterServicePlan, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterserviceplansResource, "status", clusterServicePlan), &servicecatalog.ClusterServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ClusterServicePlan), err
}

// Delete takes name of the clusterServicePlan and deletes it. Returns an error if one occurs.
func (c *FakeClusterServicePlans) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterserviceplansResource, name), &servicecatalog.ClusterServicePlan{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterServicePlans) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterserviceplansResource, listOptions)

	_, err := c.Fake.Invokes(action, &servicecatalog.ClusterServicePlanList{})
	return err
}

// Patch applies the patch and returns the patched clusterServicePlan.
func (c *FakeClusterServicePlans) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ClusterServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterserviceplansResource, name, data, subresources...), &servicecatalog.ClusterServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ClusterServicePlan), err
}
