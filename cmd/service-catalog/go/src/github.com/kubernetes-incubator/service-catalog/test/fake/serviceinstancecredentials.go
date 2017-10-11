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
	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"

	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1beta1typed "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
)

// ServiceBindings is a wrapper around the generated fake
// ServiceBindings that clones the ServiceBinding objects
// being passed to UpdateStatus. This is a workaround until the generated fake
// clientset does its own copying.
type ServiceBindings struct {
	v1beta1typed.ServiceBindingInterface
}

func (c *ServiceBindings) Create(serviceInstance *v1beta1.ServiceBinding) (result *v1beta1.ServiceBinding, err error) {
	return c.ServiceBindingInterface.Create(serviceInstance)
}

func (c *ServiceBindings) Update(serviceInstance *v1beta1.ServiceBinding) (result *v1beta1.ServiceBinding, err error) {
	return c.ServiceBindingInterface.Update(serviceInstance)
}

func (c *ServiceBindings) UpdateStatus(serviceInstance *v1beta1.ServiceBinding) (*v1beta1.ServiceBinding, error) {
	clone, err := api.Scheme.DeepCopy(serviceInstance)
	if err != nil {
		return nil, err
	}
	instanceCopy := clone.(*v1beta1.ServiceBinding)
	_, err = c.ServiceBindingInterface.UpdateStatus(instanceCopy)
	return serviceInstance, err
}

func (c *ServiceBindings) Delete(name string, options *v1.DeleteOptions) error {
	return c.ServiceBindingInterface.Delete(name, options)
}

func (c *ServiceBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.ServiceBindingInterface.DeleteCollection(options, listOptions)
}

func (c *ServiceBindings) Get(name string, options v1.GetOptions) (result *v1beta1.ServiceBinding, err error) {
	return c.ServiceBindingInterface.Get(name, options)
}

func (c *ServiceBindings) List(opts v1.ListOptions) (result *v1beta1.ServiceBindingList, err error) {
	return c.ServiceBindingInterface.List(opts)
}

// Watch returns a watch.Interface that watches the requested serviceInstances.
func (c *ServiceBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.ServiceBindingInterface.Watch(opts)
}

// Patch applies the patch and returns the patched serviceInstance.
func (c *ServiceBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ServiceBinding, err error) {
	return c.ServiceBindingInterface.Patch(name, pt, data, subresources...)
}
