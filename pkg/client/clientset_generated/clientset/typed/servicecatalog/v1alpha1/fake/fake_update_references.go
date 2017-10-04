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
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	testing "k8s.io/client-go/testing"
)

func (c *FakeServiceInstances) UpdateReferences(serviceInstance *v1alpha1.ServiceInstance) (*v1alpha1.ServiceInstance, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(serviceinstancesResource, "reference", c.ns, serviceInstance), serviceInstance)

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ServiceInstance), err
}
