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
	"k8s.io/client-go/pkg/api"

	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	v1alpha1typed "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
)

// ServiceInstanceCredentials is a wrapper around the generated fake
// ServiceInstanceCredentials that clones the ServiceInstanceCredential objects
// being passed to UpdateStatus. This is a workaround until the generated fake
// clientset does its own copying.
type ServiceInstanceCredentials struct {
	v1alpha1typed.ServiceInstanceCredentialInterface
}

func (c *ServiceInstanceCredentials) Create(serviceInstance *v1alpha1.ServiceInstanceCredential) (result *v1alpha1.ServiceInstanceCredential, err error) {
	return c.ServiceInstanceCredentialInterface.Create(serviceInstance)
}

func (c *ServiceInstanceCredentials) Update(serviceInstance *v1alpha1.ServiceInstanceCredential) (result *v1alpha1.ServiceInstanceCredential, err error) {
	return c.ServiceInstanceCredentialInterface.Update(serviceInstance)
}

func (c *ServiceInstanceCredentials) UpdateStatus(serviceInstance *v1alpha1.ServiceInstanceCredential) (*v1alpha1.ServiceInstanceCredential, error) {
	clone, err := api.Scheme.DeepCopy(serviceInstance)
	if err != nil {
		return nil, err
	}
	instanceCopy := clone.(*v1alpha1.ServiceInstanceCredential)
	_, err = c.ServiceInstanceCredentialInterface.UpdateStatus(instanceCopy)
	return serviceInstance, err
}

func (c *ServiceInstanceCredentials) Delete(name string, options *v1.DeleteOptions) error {
	return c.ServiceInstanceCredentialInterface.Delete(name, options)
}

func (c *ServiceInstanceCredentials) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.ServiceInstanceCredentialInterface.DeleteCollection(options, listOptions)
}

func (c *ServiceInstanceCredentials) Get(name string, options v1.GetOptions) (result *v1alpha1.ServiceInstanceCredential, err error) {
	return c.ServiceInstanceCredentialInterface.Get(name, options)
}

func (c *ServiceInstanceCredentials) List(opts v1.ListOptions) (result *v1alpha1.ServiceInstanceCredentialList, err error) {
	return c.ServiceInstanceCredentialInterface.List(opts)
}

// Watch returns a watch.Interface that watches the requested serviceInstances.
func (c *ServiceInstanceCredentials) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.ServiceInstanceCredentialInterface.Watch(opts)
}

// Patch applies the patch and returns the patched serviceInstance.
func (c *ServiceInstanceCredentials) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ServiceInstanceCredential, err error) {
	return c.ServiceInstanceCredentialInterface.Patch(name, pt, data, subresources...)
}
