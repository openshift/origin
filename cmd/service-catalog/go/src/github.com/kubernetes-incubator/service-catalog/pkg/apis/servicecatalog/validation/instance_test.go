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

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

func TestValidateInstance(t *testing.T) {
	cases := []struct {
		name     string
		instance *servicecatalog.Instance
		valid    bool
	}{
		{
			name: "valid",
			instance: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan",
				},
			},
			valid: true,
		},
		{
			name: "missing namespace",
			instance: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-instance",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "test-plan",
				},
			},
			valid: false,
		},
		{
			name: "missing serviceClassName",
			instance: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					PlanName: "test-plan",
				},
			},
			valid: false,
		},
		{
			name: "invalid serviceClassName",
			instance: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "oing20&)*^&",
					PlanName:         "test-plan",
				},
			},
			valid: false,
		},
		{
			name: "missing planName",
			instance: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
				},
			},
			valid: false,
		},
		{
			name: "invalid planName",
			instance: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "9651.JVHbebe",
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateInstance(tc.instance)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
