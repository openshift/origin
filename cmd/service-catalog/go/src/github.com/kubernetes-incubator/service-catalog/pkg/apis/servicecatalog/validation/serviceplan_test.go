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

func validServicePlan() *servicecatalog.ServicePlan {
	return &servicecatalog.ServicePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plan",
		},
		Spec: servicecatalog.ServicePlanSpec{
			ServiceBrokerName: "test-broker",
			ExternalName:      "test-plan",
			ExternalID:        "40d-0983-1b89",
			Description:       "plan description",
			ServiceClassRef: v1.LocalObjectReference{
				Name: "test-service-class",
			},
		},
	}
}

func TestValidateServicePlan(t *testing.T) {
	testCases := []struct {
		name        string
		servicePlan *servicecatalog.ServicePlan
		valid       bool
	}{
		{
			name:        "valid servicePlan",
			servicePlan: validServicePlan(),
			valid:       true,
		},
		{
			name: "missing name",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Name = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "bad name",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Name = "X"
				return s
			}(),
			valid: false,
		},
		{
			name: "bad externalName",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ExternalName = "X"
				return s
			}(),
			valid: false,
		},
		{
			name: "missing serviceBrokerName",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ServiceBrokerName = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "missing externalName",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ExternalName = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "missing external id",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ExternalID = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "external id too long",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ExternalID = "1234567890123456789012345678901234567890123456789012345678901234"
				return s
			}(),
			valid: false,
		},
		{
			name: "missing description",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.Description = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "missing serviceclass reference",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ServiceClassRef.Name = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "bad serviceclass reference name",
			servicePlan: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ServiceClassRef.Name = "%"
				return s
			}(),
			valid: false,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateServicePlan(tc.servicePlan)
			t.Log(errs)
			if len(errs) != 0 && tc.valid {
				t.Errorf("%v: unexpected error: %v", tc.name, errs)
			} else if len(errs) == 0 && !tc.valid {
				t.Errorf("%v: unexpected success", tc.name)
			}
		})
	}
}

func TestValidateServicePlanUpdate(t *testing.T) {
	testCases := []struct {
		name  string
		old   *servicecatalog.ServicePlan
		new   *servicecatalog.ServicePlan
		valid bool
	}{
		{
			name:  "valid servicePlan update same content",
			old:   validServicePlan(),
			new:   validServicePlan(),
			valid: true,
		},
		{
			name: "valid servicePlan update different content",
			old:  validServicePlan(),
			new: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.Description = "a new description cause it changed"
				return s
			}(),
			valid: true,
		},
		{
			name: "servicePlan changing external ID",
			old:  validServicePlan(),
			new: func() *servicecatalog.ServicePlan {
				s := validServicePlan()
				s.Spec.ExternalID = "something-else"
				return s
			}(),
			valid: false,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateServicePlanUpdate(tc.new, tc.old)
			t.Log(errs)
			if len(errs) != 0 && tc.valid {
				t.Errorf("%v: unexpected error: %v", tc.name, errs)
			} else if len(errs) == 0 && !tc.valid {
				t.Errorf("%v: unexpected success", tc.name)
			}
		})
	}
}
