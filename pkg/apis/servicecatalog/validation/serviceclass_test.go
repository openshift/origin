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

func validServiceClass() *servicecatalog.ServiceClass {
	return &servicecatalog.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-serviceclass",
		},
		Bindable:    true,
		BrokerName:  "test-broker",
		ExternalID:  "1234-4354a-49b",
		Description: "service description",
		Plans: []servicecatalog.ServicePlan{
			{
				Name:        "test-plan",
				ExternalID:  "40d-0983-1b89",
				Description: "plan description",
			},
		},
	}
}

func TestValidateServiceClass(t *testing.T) {
	cases := []struct {
		name         string
		serviceClass *servicecatalog.ServiceClass
		valid        bool
	}{
		{
			name:         "valid serviceClass",
			serviceClass: validServiceClass(),
			valid:        true,
		},
		{
			name: "valid serviceClass - plan with underscore in name",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Plans[0].Name = "test_plan"
				return s
			}(),
			valid: true,
		},
		{
			name: "valid serviceClass - uppercase in GUID",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.ExternalID = "40D-0983-1b89"
				return s
			}(),
			valid: true,
		},
		{
			name: "valid serviceClass - period in GUID",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.ExternalID = "4315f5e1-0139-4ecf-9706-9df0aff33e5a.plan-name"
				return s
			}(),
			valid: true,
		},
		{
			name: "invalid serviceClass - has namespace",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Namespace = "test-ns"
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - missing guid",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.ExternalID = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - invalid guid",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.ExternalID = "1234-4354a\\%-49b"
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - missing description",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Description = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - invalid plan name",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Plans[0].Name = "test-plan.oops"
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - invalid plan guid",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Plans[0].ExternalID = "40d-0983-1b89-★"
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - missing plan guid",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Plans[0].ExternalID = "40d-0983-1b89-★"
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - missing plan description",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Plans[0].Description = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClass - no plans",
			serviceClass: func() *servicecatalog.ServiceClass {
				s := validServiceClass()
				s.Plans = nil
				return s
			}(),
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateServiceClass(tc.serviceClass)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
