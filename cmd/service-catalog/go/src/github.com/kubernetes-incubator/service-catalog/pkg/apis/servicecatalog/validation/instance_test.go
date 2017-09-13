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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

func TestValidateServiceInstance(t *testing.T) {
	cases := []struct {
		name     string
		instance *servicecatalog.ServiceInstance
		valid    bool
	}{
		{
			name: "valid",
			instance: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan",
				},
			},
			valid: true,
		},
		{
			name: "missing namespace",
			instance: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-instance",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "test-plan",
				},
			},
			valid: false,
		},
		{
			name: "missing serviceClassName",
			instance: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					PlanName: "test-plan",
				},
			},
			valid: false,
		},
		{
			name: "invalid serviceClassName",
			instance: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "oing20&)*^&",
					PlanName:         "test-plan",
				},
			},
			valid: false,
		},
		{
			name: "missing planName",
			instance: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
				},
			},
			valid: false,
		},
		{
			name: "invalid planName",
			instance: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "9651.JVHbebe",
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateServiceInstance(tc.instance)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}

func TestValidateServiceInstanceUpdate(t *testing.T) {
	cases := []struct {
		name  string
		old   *servicecatalog.ServiceInstance
		new   *servicecatalog.ServiceInstance
		valid bool
		err   string // Error string to match against if error expected
	}{
		{
			name: "no update with async op in progress",
			old: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan",
				},
				Status: servicecatalog.ServiceInstanceStatus{
					AsyncOpInProgress: true,
				},
			},
			new: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan-2",
				},
				Status: servicecatalog.ServiceInstanceStatus{
					AsyncOpInProgress: true,
				},
			},
			valid: false,
			err:   "Another operation for this service instance is in progress",
		},
		{
			name: "allow update with no async op in progress",
			old: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan",
				},
				Status: servicecatalog.ServiceInstanceStatus{
					AsyncOpInProgress: false,
				},
			},
			new: &servicecatalog.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.ServiceInstanceSpec{
					ServiceClassName: "test-serviceclass",
					// TODO(vaikas): This does not actually update
					// spec yet, but once it does, validate it changes.
					PlanName: "Test-Plan-2",
				},
				Status: servicecatalog.ServiceInstanceStatus{
					AsyncOpInProgress: false,
				},
			},
			valid: true,
			err:   "",
		},
	}

	for _, tc := range cases {
		errs := ValidateServiceInstanceUpdate(tc.new, tc.old)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
		if !tc.valid {
			for _, err := range errs {
				if !strings.Contains(err.Detail, tc.err) {
					t.Errorf("Error %q did not contain expected message %q", err.Detail, tc.err)
				}
			}
		}
	}
}

func TestValidateServiceInstanceStatusUpdate(t *testing.T) {
	cases := []struct {
		name  string
		old   *servicecatalog.ServiceInstanceStatus
		new   *servicecatalog.ServiceInstanceStatus
		valid bool
		err   string // Error string to match against if error expected
	}{
		{
			name: "Start async op",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
			},
			valid: true,
			err:   "",
		},
		{
			name: "Complete async op",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
			},
			valid: true,
			err:   "",
		},
		{
			name: "ServiceInstanceConditionReady can not be true if async is ongoing",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
			},
			valid: false,
			err:   "async operation is in progress",
		},
		{
			name: "ServiceInstanceConditionReady can be true if async is completed",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition ready status during async",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition ready status during async false",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition to ready status and finish async op",
			old: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: true,
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
	}

	for _, tc := range cases {
		old := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				ServiceClassName: "test-serviceclass",
				PlanName:         "Test-Plan",
			},
			Status: *tc.old,
		}
		new := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				ServiceClassName: "test-serviceclass",
				PlanName:         "Test-Plan",
			},
			Status: *tc.new,
		}

		errs := ValidateServiceInstanceStatusUpdate(new, old)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
		if !tc.valid {
			for _, err := range errs {
				if !strings.Contains(err.Detail, tc.err) {
					t.Errorf("Error %q did not contain expected message %q", err.Detail, tc.err)
				}
			}
		}
	}
}
