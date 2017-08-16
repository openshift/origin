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
	servicecatalog "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeServiceClasses implements ServiceClassInterface
type FakeServiceClasses struct {
	Fake *FakeServicecatalog
}

var serviceclassesResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "", Resource: "serviceclasses"}

var serviceclassesKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "", Kind: "ServiceClass"}

func (c *FakeServiceClasses) Create(serviceClass *servicecatalog.ServiceClass) (result *servicecatalog.ServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(serviceclassesResource, serviceClass), &servicecatalog.ServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceClass), err
}

func (c *FakeServiceClasses) Update(serviceClass *servicecatalog.ServiceClass) (result *servicecatalog.ServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(serviceclassesResource, serviceClass), &servicecatalog.ServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceClass), err
}

func (c *FakeServiceClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(serviceclassesResource, name), &servicecatalog.ServiceClass{})
	return err
}

func (c *FakeServiceClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(serviceclassesResource, listOptions)

	_, err := c.Fake.Invokes(action, &servicecatalog.ServiceClassList{})
	return err
}

func (c *FakeServiceClasses) Get(name string, options v1.GetOptions) (result *servicecatalog.ServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(serviceclassesResource, name), &servicecatalog.ServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceClass), err
}

func (c *FakeServiceClasses) List(opts v1.ListOptions) (result *servicecatalog.ServiceClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(serviceclassesResource, serviceclassesKind, opts), &servicecatalog.ServiceClassList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &servicecatalog.ServiceClassList{}
	for _, item := range obj.(*servicecatalog.ServiceClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested serviceClasses.
func (c *FakeServiceClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(serviceclassesResource, opts))
}

// Patch applies the patch and returns the patched serviceClass.
func (c *FakeServiceClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServiceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(serviceclassesResource, name, data, subresources...), &servicecatalog.ServiceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServiceClass), err
}
