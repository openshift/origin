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

func validServiceInstanceCredential() *servicecatalog.ServiceInstanceCredential {
	return &servicecatalog.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding",
			Namespace: "test-ns",
		},
		Spec: servicecatalog.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{
				Name: "test-instance",
			},
			SecretName: "test-secret",
		},
	}
}

func TestValidateServiceInstanceCredential(t *testing.T) {
	cases := []struct {
		name    string
		binding *servicecatalog.ServiceInstanceCredential
		valid   bool
	}{
		{
			name:    "valid",
			binding: validServiceInstanceCredential(),
			valid:   true,
		},
		{
			name: "missing namespace",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Namespace = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "missing instance name",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Spec.ServiceInstanceRef.Name = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "invalid instance name",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Spec.ServiceInstanceRef.Name = "test-instance-)*!"
				return b
			}(),
			valid: false,
		},
		{
			name: "missing secretName",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Spec.SecretName = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "invalid secretName",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Spec.SecretName = "T_T"
				return b
			}(),
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateServiceInstanceCredential(tc.binding)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
