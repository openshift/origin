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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

const externalClusterServiceClassName = "test-serviceclass"
const externalClusterServicePlanName = "test-plan"

func validServiceInstanceForCreate() *servicecatalog.ServiceInstance {
	return &servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-instance",
			Namespace:  "test-ns",
			Generation: 1,
		},
		Spec: servicecatalog.ServiceInstanceSpec{
			PlanReference: servicecatalog.PlanReference{
				ExternalClusterServiceClassName: externalClusterServiceClassName,
				ExternalClusterServicePlanName:  externalClusterServicePlanName,
			},
		},
	}
}

func validServiceInstance() *servicecatalog.ServiceInstance {
	instance := validServiceInstanceForCreate()
	instance.Spec.ClusterServiceClassRef = &corev1.ObjectReference{}
	instance.Spec.ClusterServicePlanRef = &corev1.ObjectReference{}
	return instance
}

func validServiceInstanceWithInProgressProvision() *servicecatalog.ServiceInstance {
	instance := validServiceInstance()
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationProvision
	now := metav1.Now()
	instance.Status.OperationStartTime = &now
	instance.Status.InProgressProperties = validServiceInstancePropertiesState()
	return instance
}

func validServiceInstancePropertiesState() *servicecatalog.ServiceInstancePropertiesState {
	return &servicecatalog.ServiceInstancePropertiesState{
		Parameters:         &runtime.RawExtension{Raw: []byte("a: 1\nb: \"2\"")},
		ParametersChecksum: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

func TestValidateServiceInstance(t *testing.T) {
	cases := []struct {
		name     string
		instance *servicecatalog.ServiceInstance
		create   bool
		valid    bool
	}{
		{
			name:     "valid",
			instance: validServiceInstance(),
			valid:    true,
		},
		{
			name: "missing namespace",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Namespace = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "missing serviceClassName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ExternalClusterServiceClassName = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClassName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ExternalClusterServiceClassName = "oing20&)*^&"
				return i
			}(),
			valid: false,
		},
		{
			name: "missing planName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ExternalClusterServicePlanName = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "invalid planName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ExternalClusterServicePlanName = "9651.JVHbebe"
				return i
			}(),
			valid: false,
		},
		{
			name:     "valid with in-progress provision",
			instance: validServiceInstanceWithInProgressProvision(),
			valid:    true,
		},
		{
			name: "valid with in-progress update",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationUpdate
				return i
			}(),
			valid: true,
		},
		{
			name: "valid with in-progress deprovision",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationDeprovision
				i.Status.InProgressProperties = nil
				return i
			}(),
			valid: true,
		},
		{
			name: "invalid current operation",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperation("bad-operation")
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress without updated generation",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.ReconciledGeneration = i.Generation
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress with missing OperationStartTime",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.OperationStartTime = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present OperationStartTime",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				now := metav1.Now()
				i.Status.OperationStartTime = &now
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress with condition ready/true",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.Conditions = []servicecatalog.ServiceInstanceCondition{
					{
						Type:   servicecatalog.ServiceInstanceConditionReady,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress with condition ready/false",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.Conditions = []servicecatalog.ServiceInstanceCondition{
					{
						Type:   servicecatalog.ServiceInstanceConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
				}
				return i
			}(),
			valid: true,
		},
		{
			name: "in-progress provision with missing InProgressParameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress update with missing InProgressParameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationUpdate
				i.Status.InProgressProperties = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present InProgressParameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.InProgressProperties = validServiceInstancePropertiesState()
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress deprovision with present InProgressParameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationDeprovision
				return i
			}(),
			valid: false,
		},
		{
			name: "valid in-progress properties with no parameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.Parameters = nil
				i.Status.InProgressProperties.ParametersChecksum = ""
				return i
			}(),
			valid: true,
		},
		{
			name: "in-progress properties parameters with missing parameters checksum",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.ParametersChecksum = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum with missing parameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.Parameters = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters with missing raw",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.Parameters.Raw = []byte{}
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters with malformed yaml",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.Parameters.Raw = []byte("bad yaml")
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum too small",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.ParametersChecksum = "0123456"
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum malformed",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.ParametersChecksum = "not hex"
				return i
			}(),
			valid: false,
		},
		{
			name: "valid external properties",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				return i
			}(),
			valid: true,
		},
		{
			name: "valid external properties with no parameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.Parameters = nil
				i.Status.ExternalProperties.ParametersChecksum = ""
				return i
			}(),
			valid: true,
		},
		{
			name: "external properties parameters with missing parameters checksum",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.ParametersChecksum = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum with missing parameters",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.Parameters = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "external properties parameters with missing raw",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.Parameters.Raw = []byte{}
				return i
			}(),
			valid: false,
		},
		{
			name: "external properties parameters with malformed yaml",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.Parameters.Raw = []byte("bad yaml")
				return i
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum too small",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.ParametersChecksum = "0123456"
				return i
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum malformed",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.ParametersChecksum = "not hex"
				return i
			}(),
			valid: false,
		},
		{
			name:     "valid create",
			instance: validServiceInstanceForCreate(),
			create:   true,
			valid:    true,
		},
		{
			name: "create with operation in-progress",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceForCreate()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationProvision
				return i
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "create with invalid reconciled generation",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceForCreate()
				i.Status.ReconciledGeneration = 1
				return i
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "update with invalid reconciled generation",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ReconciledGeneration = 2
				return i
			}(),
			create: false,
			valid:  false,
		},
		{
			name: "in-progress operation with missing service class ref",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Spec.ClusterServiceClassRef = nil
				return i
			}(),
			create: false,
			valid:  false,
		},
		{
			name: "in-progress operation with missing service plan ref",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Spec.ClusterServicePlanRef = nil
				return i
			}(),
			create: false,
			valid:  false,
		},
	}

	for _, tc := range cases {
		errs := internalValidateServiceInstance(tc.instance, tc.create)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}

func TestInternalValidateServiceInstanceUpdateAllowed(t *testing.T) {
	cases := []struct {
		name              string
		newSpecChange     bool
		onGoingSpecChange bool
		valid             bool
	}{
		{
			name:              "spec change when no on-going spec change",
			newSpecChange:     true,
			onGoingSpecChange: false,
			valid:             true,
		},
		{
			name:              "spec change when on-going spec change",
			newSpecChange:     true,
			onGoingSpecChange: true,
			valid:             false,
		},
		{
			name:              "meta change when no on-going spec change",
			newSpecChange:     false,
			onGoingSpecChange: false,
			valid:             true,
		},
		{
			name:              "meta change when on-going spec change",
			newSpecChange:     false,
			onGoingSpecChange: true,
			valid:             true,
		},
	}

	for _, tc := range cases {
		oldInstance := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ExternalClusterServiceClassName: externalClusterServiceClassName,
					ExternalClusterServicePlanName:  externalClusterServicePlanName,
				},
			},
		}
		if tc.onGoingSpecChange {
			oldInstance.Generation = 2
		} else {
			oldInstance.Generation = 1
		}
		oldInstance.Status.ReconciledGeneration = 1

		newInstance := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ExternalClusterServiceClassName: "test-serviceclass",
					ExternalClusterServicePlanName:  "test-plan",
				},
			},
		}
		if tc.newSpecChange {
			newInstance.Generation = oldInstance.Generation + 1
		} else {
			newInstance.Generation = oldInstance.Generation
		}
		newInstance.Status.ReconciledGeneration = 1

		errs := internalValidateServiceInstanceUpdateAllowed(newInstance, oldInstance)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}

func TestInternalValidateServiceInstanceUpdateAllowedForPlanChange(t *testing.T) {
	cases := []struct {
		name       string
		oldPlan    string
		newPlan    string
		newPlanRef *corev1.ObjectReference
		valid      bool
	}{
		{
			name:       "valid plan change",
			oldPlan:    "old-plan",
			newPlan:    "new-plan",
			newPlanRef: nil,
			valid:      true,
		},
		{
			name:       "plan ref not cleared",
			oldPlan:    "old-plan",
			newPlan:    "new-plan",
			newPlanRef: &corev1.ObjectReference{},
			valid:      false,
		},
		{
			name:       "no plan change",
			oldPlan:    "plan-name",
			newPlan:    "plan-name",
			newPlanRef: &corev1.ObjectReference{},
			valid:      true,
		},
	}

	for _, tc := range cases {
		oldInstance := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ExternalClusterServiceClassName: "test-serviceclass",
					ExternalClusterServicePlanName:  tc.oldPlan,
				},
				ClusterServiceClassRef: &corev1.ObjectReference{},
				ClusterServicePlanRef:  &corev1.ObjectReference{},
			},
		}

		newInstance := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ExternalClusterServiceClassName: externalClusterServiceClassName,
					ExternalClusterServicePlanName:  tc.newPlan,
				},
				ClusterServiceClassRef: &corev1.ObjectReference{},
				ClusterServicePlanRef:  tc.newPlanRef,
			},
		}

		errs := internalValidateServiceInstanceUpdateAllowed(newInstance, oldInstance)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}

func TestValidateServiceInstanceStatusUpdate(t *testing.T) {
	now := metav1.Now()
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
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				AsyncOpInProgress:    true,
			},
			valid: true,
			err:   "",
		},
		{
			name: "Complete async op",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				AsyncOpInProgress:    true,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
			},
			valid: true,
			err:   "",
		},
		{
			name: "ServiceInstanceConditionReady can not be true if operation is ongoing",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation: "",
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
			},
			valid: false,
			err:   "operation in progress",
		},
		{
			name: "ServiceInstanceConditionReady can be true if operation is completed",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation: "",
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update non-ready instance condition during operation",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				Conditions:           []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				Conditions:           []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update non-ready instance condition outside of operation",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation: "",
				Conditions:       []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation: "",
				Conditions:       []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update instance condition to ready status and finish operation",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: &servicecatalog.ServiceInstancePropertiesState{},
				Conditions:           []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation: "",
				Conditions:       []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
			},
			valid: true,
			err:   "",
		},
	}

	for _, tc := range cases {
		old := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-instance",
				Namespace:  "test-ns",
				Generation: 2,
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ExternalClusterServiceClassName: externalClusterServiceClassName,
					ExternalClusterServicePlanName:  externalClusterServicePlanName,
				},
				ClusterServiceClassRef: &corev1.ObjectReference{},
				ClusterServicePlanRef:  &corev1.ObjectReference{},
			},
			Status: *tc.old,
		}
		old.Status.ReconciledGeneration = 1
		new := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-instance",
				Namespace:  "test-ns",
				Generation: 2,
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ExternalClusterServiceClassName: externalClusterServiceClassName,
					ExternalClusterServicePlanName:  externalClusterServicePlanName,
				},
				ClusterServiceClassRef: &corev1.ObjectReference{},
				ClusterServicePlanRef:  &corev1.ObjectReference{},
			},
			Status: *tc.new,
		}
		new.Status.ReconciledGeneration = 1

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
					t.Errorf("%v: Error %q did not contain expected message %q", tc.name, err.Detail, tc.err)
				}
			}
		}
	}
}

func TestValidateServiceInstanceReferencesUpdate(t *testing.T) {
	cases := []struct {
		name  string
		old   *servicecatalog.ServiceInstance
		new   *servicecatalog.ServiceInstance
		valid bool
	}{
		{
			name: "valid class and plan update",
			old: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServiceClassRef = nil
				i.Spec.ClusterServicePlanRef = nil
				return i
			}(),
			new:   validServiceInstance(),
			valid: true,
		},
		{
			name: "invalid class update",
			old:  validServiceInstance(),
			new: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServiceClassRef = &corev1.ObjectReference{
					Name: "new-class-name",
				}
				return i
			}(),
			valid: false,
		},
		{
			name: "direct update to plan ref",
			old:  validServiceInstance(),
			new: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServicePlanRef = &corev1.ObjectReference{
					Name: "new-plan-name",
				}
				return i
			}(),
			valid: false,
		},
		{
			name: "valid plan update from name change",
			old: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServicePlanRef = nil
				return i
			}(),
			new: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServicePlanRef = &corev1.ObjectReference{
					Name: "new-plan-name",
				}
				return i
			}(),
			valid: true,
		},
		{
			name:  "in-progress operation",
			old:   validServiceInstanceWithInProgressProvision(),
			new:   validServiceInstanceWithInProgressProvision(),
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateServiceInstanceReferencesUpdate(tc.new, tc.old)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
