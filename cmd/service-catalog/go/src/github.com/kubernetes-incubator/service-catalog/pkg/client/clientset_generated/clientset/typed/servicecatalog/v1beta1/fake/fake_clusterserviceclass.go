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

// FakeClusterServiceClasses implements ClusterServiceClassInterface
type FakeClusterServiceClasses struct {
	Fake *FakeServicecatalogV1beta1
}

var clusterserviceclassesResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "v1beta1", Resource: "clusterserviceclasses"}

var clusterserviceclassesKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "v1beta1", Kind: "ClusterServiceClass"}

// Get takes name of the clusterServiceClass, and returns the corresponding clusterServiceClass object, and an error if there is any.
func (c *FakeClusterServiceClasses) Get(name string, options v1.GetOptions) (result *v1beta1.ClusterServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterserviceclassesResource, name), &v1beta1.ClusterServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceClass), err
}

// List takes label and field selectors, and returns the list of ClusterServiceClasses that match those selectors.
func (c *FakeClusterServiceClasses) List(opts v1.ListOptions) (result *v1beta1.ClusterServiceClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterserviceclassesResource, clusterserviceclassesKind, opts), &v1beta1.ClusterServiceClassList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ClusterServiceClassList{}
	for _, item := range obj.(*v1beta1.ClusterServiceClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterServiceClasses.
func (c *FakeClusterServiceClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterserviceclassesResource, opts))
}

// Create takes the representation of a clusterServiceClass and creates it.  Returns the server's representation of the clusterServiceClass, and an error, if there is any.
func (c *FakeClusterServiceClasses) Create(clusterServiceClass *v1beta1.ClusterServiceClass) (result *v1beta1.ClusterServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterserviceclassesResource, clusterServiceClass), &v1beta1.ClusterServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceClass), err
}

// Update takes the representation of a clusterServiceClass and updates it. Returns the server's representation of the clusterServiceClass, and an error, if there is any.
func (c *FakeClusterServiceClasses) Update(clusterServiceClass *v1beta1.ClusterServiceClass) (result *v1beta1.ClusterServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterserviceclassesResource, clusterServiceClass), &v1beta1.ClusterServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceClass), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterServiceClasses) UpdateStatus(clusterServiceClass *v1beta1.ClusterServiceClass) (*v1beta1.ClusterServiceClass, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterserviceclassesResource, "status", clusterServiceClass), &v1beta1.ClusterServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceClass), err
}

// Delete takes name of the clusterServiceClass and deletes it. Returns an error if one occurs.
func (c *FakeClusterServiceClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterserviceclassesResource, name), &v1beta1.ClusterServiceClass{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterServiceClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterserviceclassesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1beta1.ClusterServiceClassList{})
	return err
}

// Patch applies the patch and returns the patched clusterServiceClass.
func (c *FakeClusterServiceClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ClusterServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterserviceclassesResource, name, data, subresources...), &v1beta1.ClusterServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ClusterServiceClass), err
}
