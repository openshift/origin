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

package binding

import (
	"testing"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/unversioned"
)

func bindingWithFalseReadyCondition() *servicecatalog.Binding {
	return &servicecatalog.Binding{
		Spec: servicecatalog.BindingSpec{
			InstanceRef: v1.LocalObjectReference{
				Name: "some-string",
			},
		},
		Status: servicecatalog.BindingStatus{
			Conditions: []servicecatalog.BindingCondition{
				{
					Type:   servicecatalog.BindingConditionReady,
					Status: servicecatalog.ConditionFalse,
				},
			},
		},
	}
}

func bindingWithTrueReadyCondition() *servicecatalog.Binding {
	return &servicecatalog.Binding{
		Spec: servicecatalog.BindingSpec{
			InstanceRef: v1.LocalObjectReference{
				Name: "some-string",
			},
		},
		Status: servicecatalog.BindingStatus{
			Conditions: []servicecatalog.BindingCondition{
				{
					Type:   servicecatalog.BindingConditionReady,
					Status: servicecatalog.ConditionTrue,
				},
			},
		},
	}
}

func TestValidateUpdateStatusPrepareForUpdate(t *testing.T) {
	cases := []struct {
		name                string
		old                 *servicecatalog.Binding
		newer               *servicecatalog.Binding
		shouldChecksum      bool
		checksumShouldBeSet bool
	}{
		{
			name:                "not ready -> not ready",
			old:                 bindingWithFalseReadyCondition(),
			newer:               bindingWithFalseReadyCondition(),
			shouldChecksum:      false,
			checksumShouldBeSet: false,
		},
		{
			name: "not ready -> not ready, checksum already set",
			old: func() *servicecatalog.Binding {
				b := bindingWithFalseReadyCondition()
				cs := "22081-9471-471"
				b.Status.Checksum = &cs
				return b
			}(),
			newer:               bindingWithFalseReadyCondition(),
			shouldChecksum:      false,
			checksumShouldBeSet: true,
		},
		{
			name:           "not ready -> ready",
			old:            bindingWithFalseReadyCondition(),
			newer:          bindingWithTrueReadyCondition(),
			shouldChecksum: true,
		},
	}

	for _, tc := range cases {
		strategy := bindingStatusUpdateStrategy
		strategy.PrepareForUpdate(nil /* api context */, tc.newer, tc.old)

		if tc.shouldChecksum {
			if tc.newer.Status.Checksum == nil {
				t.Errorf("%v: Checksum should have been set", tc.name)
				continue
			}

			if e, a := checksum.BindingSpecChecksum(tc.newer.Spec), *tc.newer.Status.Checksum; e != a {
				t.Errorf("%v: Checksum was incorrect; expected %v got %v", tc.name, e, a)
			}
		} else if tc.checksumShouldBeSet != (tc.newer.Status.Checksum != nil) {
			t.Errorf("%v: expected checksum to be populated, but was nil", tc.name)
		}
	}
}
