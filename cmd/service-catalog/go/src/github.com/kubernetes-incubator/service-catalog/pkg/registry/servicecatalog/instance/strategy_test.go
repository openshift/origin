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

package instance

import (
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/unversioned"
)

func instanceWithFalseReadyCondition() *servicecatalog.ServiceInstance {
	return &servicecatalog.ServiceInstance{
		Spec: servicecatalog.ServiceInstanceSpec{
			ServiceClassName: "test-serviceclass",
			PlanName:         "test-plan",
		},
		Status: servicecatalog.ServiceInstanceStatus{
			Conditions: []servicecatalog.ServiceInstanceCondition{
				{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				},
			},
		},
	}
}

func instanceWithTrueReadyCondition() *servicecatalog.ServiceInstance {
	return &servicecatalog.ServiceInstance{
		Spec: servicecatalog.ServiceInstanceSpec{
			ServiceClassName: "test-serviceclass",
			PlanName:         "test-plan",
		},
		Status: servicecatalog.ServiceInstanceStatus{
			Conditions: []servicecatalog.ServiceInstanceCondition{
				{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				},
			},
		},
	}
}

func TestValidateUpdateStatusPrepareForUpdate(t *testing.T) {
	cases := []struct {
		name                string
		old                 *servicecatalog.ServiceInstance
		newer               *servicecatalog.ServiceInstance
		shouldChecksum      bool
		checksumShouldBeSet bool
	}{
		{
			name:                "not ready -> not ready",
			old:                 instanceWithFalseReadyCondition(),
			newer:               instanceWithFalseReadyCondition(),
			shouldChecksum:      false,
			checksumShouldBeSet: false,
		},
		{
			name: "not ready -> not ready, checksum already set",
			old: func() *servicecatalog.ServiceInstance {
				i := instanceWithFalseReadyCondition()
				cs := "22081-9471-471"
				i.Status.Checksum = &cs
				return i
			}(),
			newer:               instanceWithFalseReadyCondition(),
			shouldChecksum:      false,
			checksumShouldBeSet: true,
		},
		{
			name:           "not ready -> ready",
			old:            instanceWithFalseReadyCondition(),
			newer:          instanceWithTrueReadyCondition(),
			shouldChecksum: true,
		},
	}

	for _, tc := range cases {
		strategy := instanceStatusUpdateStrategy
		strategy.PrepareForUpdate(nil /* api context */, tc.newer, tc.old)

		if tc.shouldChecksum {
			if tc.newer.Status.Checksum == nil {
				t.Errorf("%v: Checksum should have been set", tc.name)
				continue
			}

			if e, a := checksum.ServiceInstanceSpecChecksum(tc.newer.Spec), *tc.newer.Status.Checksum; e != a {
				t.Errorf("%v: Checksum was incorrect; expected %v got %v", tc.name, e, a)
			}
		} else if tc.checksumShouldBeSet != (tc.newer.Status.Checksum != nil) {
			t.Errorf("%v: expected checksum to be populated, but was nil", tc.name)
		}
	}
}
