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

	servicecatalogv1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
)

// ServicecatalogV1beta1 is a wrapper around the generated fake service catalog
// that clones the ServiceInstance and ServiceBinding objects being
// passed to UpdateStatus. This is a workaround until the generated fake clientset
// does its own copying.
type ServicecatalogV1beta1 struct {
	servicecatalogv1beta1.ServicecatalogV1beta1Interface
}

var _ servicecatalogv1beta1.ServicecatalogV1beta1Interface = &ServicecatalogV1beta1{}

func (c *ServicecatalogV1beta1) ClusterServiceBrokers() v1beta1.ClusterServiceBrokerInterface {
	return c.ServicecatalogV1beta1Interface.ClusterServiceBrokers()
}

func (c *ServicecatalogV1beta1) ClusterServiceClasses() v1beta1.ClusterServiceClassInterface {
	return c.ServicecatalogV1beta1Interface.ClusterServiceClasses()
}

func (c *ServicecatalogV1beta1) ServiceInstances(namespace string) v1beta1.ServiceInstanceInterface {
	serviceInstances := c.ServicecatalogV1beta1Interface.ServiceInstances(namespace)
	return &ServiceInstances{serviceInstances}
}

func (c *ServicecatalogV1beta1) ServiceBindings(namespace string) v1beta1.ServiceBindingInterface {
	serviceBindings := c.ServicecatalogV1beta1Interface.ServiceBindings(namespace)
	return &ServiceBindings{serviceBindings}
}

func (c *ServicecatalogV1beta1) ClusterServicePlans() v1beta1.ClusterServicePlanInterface {
	return c.ServicecatalogV1beta1Interface.ClusterServicePlans()
}

func (c *ServicecatalogV1beta1) RESTClient() rest.Interface {
	return c.ServicecatalogV1beta1Interface.RESTClient()
}
