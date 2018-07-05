/*
Copyright 2018 Red Hat, Inc.

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
	v1alpha2 "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeIdlers implements IdlerInterface
type FakeIdlers struct {
	Fake *FakeIdlingV1alpha2
	ns   string
}

var idlersResource = schema.GroupVersionResource{Group: "idling.openshift.io", Version: "v1alpha2", Resource: "idlers"}

var idlersKind = schema.GroupVersionKind{Group: "idling.openshift.io", Version: "v1alpha2", Kind: "Idler"}

// Get takes name of the idler, and returns the corresponding idler object, and an error if there is any.
func (c *FakeIdlers) Get(name string, options v1.GetOptions) (result *v1alpha2.Idler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(idlersResource, c.ns, name), &v1alpha2.Idler{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.Idler), err
}

// List takes label and field selectors, and returns the list of Idlers that match those selectors.
func (c *FakeIdlers) List(opts v1.ListOptions) (result *v1alpha2.IdlerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(idlersResource, idlersKind, c.ns, opts), &v1alpha2.IdlerList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha2.IdlerList{}
	for _, item := range obj.(*v1alpha2.IdlerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested idlers.
func (c *FakeIdlers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(idlersResource, c.ns, opts))

}

// Create takes the representation of a idler and creates it.  Returns the server's representation of the idler, and an error, if there is any.
func (c *FakeIdlers) Create(idler *v1alpha2.Idler) (result *v1alpha2.Idler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(idlersResource, c.ns, idler), &v1alpha2.Idler{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.Idler), err
}

// Update takes the representation of a idler and updates it. Returns the server's representation of the idler, and an error, if there is any.
func (c *FakeIdlers) Update(idler *v1alpha2.Idler) (result *v1alpha2.Idler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(idlersResource, c.ns, idler), &v1alpha2.Idler{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.Idler), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeIdlers) UpdateStatus(idler *v1alpha2.Idler) (*v1alpha2.Idler, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(idlersResource, "status", c.ns, idler), &v1alpha2.Idler{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.Idler), err
}

// Delete takes name of the idler and deletes it. Returns an error if one occurs.
func (c *FakeIdlers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(idlersResource, c.ns, name), &v1alpha2.Idler{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeIdlers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(idlersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha2.IdlerList{})
	return err
}

// Patch applies the patch and returns the patched idler.
func (c *FakeIdlers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha2.Idler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(idlersResource, c.ns, name, data, subresources...), &v1alpha2.Idler{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.Idler), err
}
