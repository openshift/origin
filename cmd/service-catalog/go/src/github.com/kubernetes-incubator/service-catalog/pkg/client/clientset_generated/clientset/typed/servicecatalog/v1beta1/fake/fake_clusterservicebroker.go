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

package fake

import (
	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterServiceBrokers implements ClusterServiceBrokerInterface
type FakeClusterServiceBrokers struct {
	Fake *FakeServicecatalogV1beta1
}

var clusterservicebrokersResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "v1beta1", Resource: "clusterservicebrokers"}

var clusterservicebrokersKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "v1beta1", Kind: "ClusterServiceBroker"}

// Get takes name of the clusterServiceBroker, and returns the corresponding clusterServiceBroker object, and an error if there is any.
func (c *FakeClusterServiceBrokers) Get(name string, options v1.GetOptions) (result *v1beta1.ClusterServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterservicebrokersResource, name), &v1beta1.ClusterServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceBroker), err
}

// List takes label and field selectors, and returns the list of ClusterServiceBrokers that match those selectors.
func (c *FakeClusterServiceBrokers) List(opts v1.ListOptions) (result *v1beta1.ClusterServiceBrokerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterservicebrokersResource, clusterservicebrokersKind, opts), &v1beta1.ClusterServiceBrokerList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ClusterServiceBrokerList{}
	for _, item := range obj.(*v1beta1.ClusterServiceBrokerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterServiceBrokers.
func (c *FakeClusterServiceBrokers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterservicebrokersResource, opts))
}

// Create takes the representation of a clusterServiceBroker and creates it.  Returns the server's representation of the clusterServiceBroker, and an error, if there is any.
func (c *FakeClusterServiceBrokers) Create(clusterServiceBroker *v1beta1.ClusterServiceBroker) (result *v1beta1.ClusterServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterservicebrokersResource, clusterServiceBroker), &v1beta1.ClusterServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceBroker), err
}

// Update takes the representation of a clusterServiceBroker and updates it. Returns the server's representation of the clusterServiceBroker, and an error, if there is any.
func (c *FakeClusterServiceBrokers) Update(clusterServiceBroker *v1beta1.ClusterServiceBroker) (result *v1beta1.ClusterServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterservicebrokersResource, clusterServiceBroker), &v1beta1.ClusterServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceBroker), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterServiceBrokers) UpdateStatus(clusterServiceBroker *v1beta1.ClusterServiceBroker) (*v1beta1.ClusterServiceBroker, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterservicebrokersResource, "status", clusterServiceBroker), &v1beta1.ClusterServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceBroker), err
}

// Delete takes name of the clusterServiceBroker and deletes it. Returns an error if one occurs.
func (c *FakeClusterServiceBrokers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterservicebrokersResource, name), &v1beta1.ClusterServiceBroker{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterServiceBrokers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterservicebrokersResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1beta1.ClusterServiceBrokerList{})
	return err
}

// Patch applies the patch and returns the patched clusterServiceBroker.
func (c *FakeClusterServiceBrokers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ClusterServiceBroker, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterservicebrokersResource, name, data, subresources...), &v1beta1.ClusterServiceBroker{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceBroker), err
}
