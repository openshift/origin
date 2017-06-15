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

func TestValidateInstanceUpdate(t *testing.T) {
	cases := []struct {
		name  string
		old   *servicecatalog.Instance
		new   *servicecatalog.Instance
		valid bool
		err   string // Error string to match against if error expected
	}{
		{
			name: "no update with async op in progress",
			old: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan",
				},
				Status: servicecatalog.InstanceStatus{
					AsyncOpInProgress: true,
				},
			},
			new: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan-2",
				},
				Status: servicecatalog.InstanceStatus{
					AsyncOpInProgress: true,
				},
			},
			valid: false,
			err:   "Another operation for this service instance is in progress",
		},
		{
			name: "allow update with no async op in progress",
			old: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					PlanName:         "Test-Plan",
				},
				Status: servicecatalog.InstanceStatus{
					AsyncOpInProgress: false,
				},
			},
			new: &servicecatalog.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-ns",
				},
				Spec: servicecatalog.InstanceSpec{
					ServiceClassName: "test-serviceclass",
					// TODO(vaikas): This does not actually update
					// spec yet, but once it does, validate it changes.
					PlanName: "Test-Plan-2",
				},
				Status: servicecatalog.InstanceStatus{
					AsyncOpInProgress: false,
				},
			},
			valid: true,
			err:   "",
		},
	}

	for _, tc := range cases {
		errs := ValidateInstanceUpdate(tc.new, tc.old)
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

func TestValidateInstanceStatusUpdate(t *testing.T) {
	cases := []struct {
		name  string
		old   *servicecatalog.InstanceStatus
		new   *servicecatalog.InstanceStatus
		valid bool
		err   string // Error string to match against if error expected
	}{
		{
			name: "Start async op",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: false,
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
			},
			valid: true,
			err:   "",
		},
		{
			name: "Complete async op",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: false,
			},
			valid: true,
			err:   "",
		},
		{
			name: "InstanceConditionReady can not be true if async is ongoing",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
				Conditions: []servicecatalog.InstanceCondition{{
					Type:   servicecatalog.InstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
				Conditions: []servicecatalog.InstanceCondition{{
					Type:   servicecatalog.InstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
			},
			valid: false,
			err:   "async operation is in progress",
		},
		{
			name: "InstanceConditionReady can be true if async is completed",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
				Conditions: []servicecatalog.InstanceCondition{{
					Type:   servicecatalog.InstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: false,
				Conditions: []servicecatalog.InstanceCondition{{
					Type:   servicecatalog.InstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition ready status during async",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
				Conditions:        []servicecatalog.InstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
				Conditions:        []servicecatalog.InstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition ready status during async false",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: false,
				Conditions:        []servicecatalog.InstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: false,
				Conditions:        []servicecatalog.InstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition to ready status and finish async op",
			old: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: true,
				Conditions:        []servicecatalog.InstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.InstanceStatus{
				AsyncOpInProgress: false,
				Conditions:        []servicecatalog.InstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
	}

	for _, tc := range cases {
		old := &servicecatalog.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.InstanceSpec{
				ServiceClassName: "test-serviceclass",
				PlanName:         "Test-Plan",
			},
			Status: *tc.old,
		}
		new := &servicecatalog.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.InstanceSpec{
				ServiceClassName: "test-serviceclass",
				PlanName:         "Test-Plan",
			},
			Status: *tc.new,
		}

		errs := ValidateInstanceStatusUpdate(new, old)
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
