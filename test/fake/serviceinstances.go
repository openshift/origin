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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	"github.com/kubernetes-incubator/service-catalog/pkg/api"

	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1beta1typed "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
)

// ServiceInstances is a wrapper around the generated fake ServiceInstances
// that clones the ServiceInstance objects being passed to UpdateStatus. This is a
// workaround until the generated fake clientset does its own copying.
type ServiceInstances struct {
	v1beta1typed.ServiceInstanceInterface
}

func (c *ServiceInstances) Create(serviceInstance *v1beta1.ServiceInstance) (result *v1beta1.ServiceInstance, err error) {
	return c.ServiceInstanceInterface.Create(serviceInstance)
}

func (c *ServiceInstances) Update(serviceInstance *v1beta1.ServiceInstance) (result *v1beta1.ServiceInstance, err error) {
	return c.ServiceInstanceInterface.Update(serviceInstance)
}

func (c *ServiceInstances) UpdateStatus(serviceInstance *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	clone, err := api.Scheme.DeepCopy(serviceInstance)
	if err != nil {
		return nil, err
	}
	instanceCopy := clone.(*v1beta1.ServiceInstance)
	_, err = c.ServiceInstanceInterface.UpdateStatus(instanceCopy)
	return serviceInstance, err
}

func (c *ServiceInstances) Delete(name string, options *v1.DeleteOptions) error {
	return c.ServiceInstanceInterface.Delete(name, options)
}

func (c *ServiceInstances) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.ServiceInstanceInterface.DeleteCollection(options, listOptions)
}

func (c *ServiceInstances) Get(name string, options v1.GetOptions) (result *v1beta1.ServiceInstance, err error) {
	return c.ServiceInstanceInterface.Get(name, options)
}

func (c *ServiceInstances) List(opts v1.ListOptions) (result *v1beta1.ServiceInstanceList, err error) {
	return c.ServiceInstanceInterface.List(opts)
}

// Watch returns a watch.Interface that watches the requested serviceInstances.
func (c *ServiceInstances) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.ServiceInstanceInterface.Watch(opts)
}

// Patch applies the patch and returns the patched serviceInstance.
func (c *ServiceInstances) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ServiceInstance, err error) {
	return c.ServiceInstanceInterface.Patch(name, pt, data, subresources...)
}
