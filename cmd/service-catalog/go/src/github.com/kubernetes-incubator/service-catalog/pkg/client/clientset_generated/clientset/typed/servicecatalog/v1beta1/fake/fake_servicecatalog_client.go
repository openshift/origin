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
	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeServicecatalogV1beta1 struct {
	*testing.Fake
}

func (c *FakeServicecatalogV1beta1) ClusterServiceBrokers() v1beta1.ClusterServiceBrokerInterface {
	return &FakeClusterServiceBrokers{c}
}

func (c *FakeServicecatalogV1beta1) ClusterServiceClasses() v1beta1.ClusterServiceClassInterface {
	return &FakeClusterServiceClasses{c}
}

func (c *FakeServicecatalogV1beta1) ClusterServicePlans() v1beta1.ClusterServicePlanInterface {
	return &FakeClusterServicePlans{c}
}

func (c *FakeServicecatalogV1beta1) ServiceBindings(namespace string) v1beta1.ServiceBindingInterface {
	return &FakeServiceBindings{c, namespace}
}

func (c *FakeServicecatalogV1beta1) ServiceInstances(namespace string) v1beta1.ServiceInstanceInterface {
	return &FakeServiceInstances{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeServicecatalogV1beta1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
