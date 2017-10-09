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
	"k8s.io/client-go/discovery"

	clientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/fake"
	servicecatalogv1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
)

// Clientset is a wrapper around the generated fake clientset that clones the
// ServiceInstance and ServiceInstanceCredential objects being passed to
// UpdateStatus. This is a workaround until the generated fake clientset does its
// own copying.
type Clientset struct {
	*servicecatalogclientset.Clientset
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.Clientset.Discovery()
}

var _ clientset.Interface = &Clientset{}

func (c *Clientset) ServicecatalogV1alpha1() servicecatalogv1alpha1.ServicecatalogV1alpha1Interface {
	return &ServicecatalogV1alpha1{c.Clientset.ServicecatalogV1alpha1()}
}

func (c *Clientset) Servicecatalog() servicecatalogv1alpha1.ServicecatalogV1alpha1Interface {
	return &ServicecatalogV1alpha1{c.Clientset.ServicecatalogV1alpha1()}
}
