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

// FakeServicePlans implements ServicePlanInterface
type FakeServicePlans struct {
	Fake *FakeServicecatalog
}

var serviceplansResource = schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "", Resource: "serviceplans"}

var serviceplansKind = schema.GroupVersionKind{Group: "servicecatalog.k8s.io", Version: "", Kind: "ServicePlan"}

func (c *FakeServicePlans) Create(servicePlan *servicecatalog.ServicePlan) (result *servicecatalog.ServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(serviceplansResource, servicePlan), &servicecatalog.ServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServicePlan), err
}

func (c *FakeServicePlans) Update(servicePlan *servicecatalog.ServicePlan) (result *servicecatalog.ServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(serviceplansResource, servicePlan), &servicecatalog.ServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServicePlan), err
}

func (c *FakeServicePlans) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(serviceplansResource, name), &servicecatalog.ServicePlan{})
	return err
}

func (c *FakeServicePlans) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(serviceplansResource, listOptions)

	_, err := c.Fake.Invokes(action, &servicecatalog.ServicePlanList{})
	return err
}

func (c *FakeServicePlans) Get(name string, options v1.GetOptions) (result *servicecatalog.ServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(serviceplansResource, name), &servicecatalog.ServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServicePlan), err
}

func (c *FakeServicePlans) List(opts v1.ListOptions) (result *servicecatalog.ServicePlanList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(serviceplansResource, serviceplansKind, opts), &servicecatalog.ServicePlanList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &servicecatalog.ServicePlanList{}
	for _, item := range obj.(*servicecatalog.ServicePlanList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested servicePlans.
func (c *FakeServicePlans) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(serviceplansResource, opts))
}

// Patch applies the patch and returns the patched servicePlan.
func (c *FakeServicePlans) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *servicecatalog.ServicePlan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(serviceplansResource, name, data, subresources...), &servicecatalog.ServicePlan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*servicecatalog.ServicePlan), err
}
