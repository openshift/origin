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
	"k8s.io/client-go/pkg/api/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

func TestValidateBinding(t *testing.T) {
	cases := []struct {
		name    string
		binding *servicecatalog.Binding
		valid   bool
	}{
		{
			name: "valid",
			binding: &servicecatalog.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.BindingSpec{
					InstanceRef: v1.LocalObjectReference{
						Name: "test-instance",
					},
					SecretName: "test-secret",
				},
			},
			valid: true,
		},
		{
			name: "missing namespace",
			binding: &servicecatalog.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: servicecatalog.BindingSpec{
					InstanceRef: v1.LocalObjectReference{
						Name: "test-instance",
					},
					SecretName: "test-secret",
				},
			},
			valid: false,
		},
		{
			name: "missing instance name",
			binding: &servicecatalog.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.BindingSpec{
					SecretName: "test-secret",
				},
			},
			valid: false,
		},
		{
			name: "invalid instance name",
			binding: &servicecatalog.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.BindingSpec{
					InstanceRef: v1.LocalObjectReference{
						Name: "test-instance-)*!",
					},
					SecretName: "test-secret",
				},
			},
			valid: false,
		},
		{
			name: "missing secretName",
			binding: &servicecatalog.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.BindingSpec{
					InstanceRef: v1.LocalObjectReference{
						Name: "test-instance",
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid secretName",
			binding: &servicecatalog.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.BindingSpec{
					InstanceRef: v1.LocalObjectReference{
						Name: "test-instance",
					},
					SecretName: "T_T",
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateBinding(tc.binding)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
