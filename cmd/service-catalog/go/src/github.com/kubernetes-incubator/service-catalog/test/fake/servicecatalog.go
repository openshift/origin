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
	rest "k8s.io/client-go/rest"

	servicecatalogv1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
)

// ServicecatalogV1alpha1 is a wrapper around the generated fake service catalog
// that clones the ServiceInstance and ServiceInstanceCredential objects being
// passed to UpdateStatus. This is a workaround until the generated fake clientset
// does its own copying.
type ServicecatalogV1alpha1 struct {
	servicecatalogv1alpha1.ServicecatalogV1alpha1Interface
}

var _ servicecatalogv1alpha1.ServicecatalogV1alpha1Interface = &ServicecatalogV1alpha1{}

func (c *ServicecatalogV1alpha1) ServiceBrokers() v1alpha1.ServiceBrokerInterface {
	return c.ServicecatalogV1alpha1Interface.ServiceBrokers()
}

func (c *ServicecatalogV1alpha1) ServiceClasses() v1alpha1.ServiceClassInterface {
	return c.ServicecatalogV1alpha1Interface.ServiceClasses()
}

func (c *ServicecatalogV1alpha1) ServiceInstances(namespace string) v1alpha1.ServiceInstanceInterface {
	serviceInstances := c.ServicecatalogV1alpha1Interface.ServiceInstances(namespace)
	return &ServiceInstances{serviceInstances}
}

func (c *ServicecatalogV1alpha1) ServiceInstanceCredentials(namespace string) v1alpha1.ServiceInstanceCredentialInterface {
	serviceInstanceCredentials := c.ServicecatalogV1alpha1Interface.ServiceInstanceCredentials(namespace)
	return &ServiceInstanceCredentials{serviceInstanceCredentials}
}

func (c *ServicecatalogV1alpha1) ServicePlans() v1alpha1.ServicePlanInterface {
	return c.ServicecatalogV1alpha1Interface.ServicePlans()
}

func (c *ServicecatalogV1alpha1) RESTClient() rest.Interface {
	return c.ServicecatalogV1alpha1Interface.RESTClient()
}
