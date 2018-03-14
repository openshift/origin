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

func validClusterServicePlan() *servicecatalog.ClusterServicePlan {
	return &servicecatalog.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plan",
		},
		Spec: servicecatalog.ClusterServicePlanSpec{
			ClusterServiceBrokerName: "test-broker",
			ExternalName:             "test-plan",
			ExternalID:               "40d-0983-1b89",
			Description:              "plan description",
			ClusterServiceClassRef: servicecatalog.ClusterObjectReference{
				Name: "test-service-class",
			},
		},
	}
}

func TestValidateClusterServicePlan(t *testing.T) {
	testCases := []struct {
		name        string
		servicePlan *servicecatalog.ClusterServicePlan
		valid       bool
	}{
		{
			name:        "valid servicePlan",
			servicePlan: validClusterServicePlan(),
			valid:       true,
		},
		{
			name: "missing name",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Name = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "bad name",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Name = "#"
				return s
			}(),
			valid: false,
		},
		{
			name: "bad externalName",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ExternalName = "#"
				return s
			}(),
			valid: false,
		},
		{
			name: "mixed case Name",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Name = "abcXYZ"
				return s
			}(),
			valid: true,
		},
		{
			name: "mixed case externalName",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ExternalName = "abcXYZ"
				return s
			}(),
			valid: true,
		},
		{
			name: "missing clusterServiceBrokerName",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ClusterServiceBrokerName = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "missing externalName",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ExternalName = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "missing external id",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ExternalID = ""
				return s
			}(),
			valid: false,
		},
		{
			// Note this is NOT due to the spec, this is due to
			// a Kubernetes limitation. So, technically this restriction
			// could cause a valid Broker to not work against Kube.
			name: "external id too long",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ExternalID = "1234567890123456789012345678901234567890123456789012345678901234"
				return s
			}(),
			valid: false,
		},
		{
			name: "missing description",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.Description = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "missing serviceclass reference",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ClusterServiceClassRef.Name = ""
				return s
			}(),
			valid: false,
		},
		{
			name: "bad serviceclass reference name",
			servicePlan: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.ClusterServiceClassRef.Name = "%"
				return s
			}(),
			valid: false,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateClusterServicePlan(tc.servicePlan)
			t.Log(errs)
			if len(errs) != 0 && tc.valid {
				t.Errorf("%v: unexpected error: %v", tc.name, errs)
			} else if len(errs) == 0 && !tc.valid {
				t.Errorf("%v: unexpected success", tc.name)
			}
		})
	}
}

func TestValidateClusterServicePlanUpdate(t *testing.T) {
	testCases := []struct {
		name  string
		old   *servicecatalog.ClusterServicePlan
		new   *servicecatalog.ClusterServicePlan
		valid bool
	}{
		{
			name:  "valid servicePlan update same content",
			old:   validClusterServicePlan(),
			new:   validClusterServicePlan(),
			valid: true,
		},
		{
			name: "valid servicePlan update different content",
			old:  validClusterServicePlan(),
			new: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
				s.Spec.Description = "a new description cause it changed"
				return s
			}(),
			valid: true,
		},
		{
			name: "servicePlan changing external ID",
			old:  validClusterServicePlan(),
			new: func() *servicecatalog.ClusterServicePlan {
				s := validClusterServicePlan()
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
			errs := ValidateClusterServicePlanUpdate(tc.new, tc.old)
			t.Log(errs)
			if len(errs) != 0 && tc.valid {
				t.Errorf("%v: unexpected error: %v", tc.name, errs)
			} else if len(errs) == 0 && !tc.valid {
				t.Errorf("%v: unexpected success", tc.name)
			}
		})
	}
}
