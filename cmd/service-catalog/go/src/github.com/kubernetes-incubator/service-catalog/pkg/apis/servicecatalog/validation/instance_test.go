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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

const (
	clusterServiceClassExternalName = "test-serviceclass"
	clusterServicePlanExternalName  = "test-plan"
	clusterServiceClassName         = "test-k8s-serviceclass"
	clusterServicePlanName          = "test-k8s-plan-name"
)

func validPlanReferenceExternal() servicecatalog.PlanReference {
	return servicecatalog.PlanReference{
		ClusterServiceClassExternalName: clusterServiceClassExternalName,
		ClusterServicePlanExternalName:  clusterServicePlanExternalName,
	}
}

func validPlanReferenceK8S() servicecatalog.PlanReference {
	return servicecatalog.PlanReference{
		ClusterServiceClassName: clusterServiceClassName,
		ClusterServicePlanName:  clusterServicePlanName,
	}
}

func validServiceInstanceForCreate() *servicecatalog.ServiceInstance {
	return &servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-instance",
			Namespace:  "test-ns",
			Generation: 1,
		},
		Spec: servicecatalog.ServiceInstanceSpec{
			PlanReference: validPlanReferenceExternal(),
		},
		Status: servicecatalog.ServiceInstanceStatus{
			DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusNotRequired,
		},
	}
}

func validServiceInstance() *servicecatalog.ServiceInstance {
	instance := validServiceInstanceForCreate()
	instance.Spec.ClusterServiceClassRef = &servicecatalog.ClusterObjectReference{}
	instance.Spec.ClusterServicePlanRef = &servicecatalog.ClusterObjectReference{}
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
		ClusterServicePlanExternalName: "plan-name",
		ClusterServicePlanExternalID:   "plan-id",
		Parameters:                     &runtime.RawExtension{Raw: []byte("a: 1\nb: \"2\"")},
		ParametersChecksum:             "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
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
			name: "missing clusterServiceClassExternalName and clusterServiceClassName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServiceClassExternalName = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "invalid serviceClassName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServiceClassExternalName = "oing20&)*^&"
				return i
			}(),
			valid: false,
		},
		{
			name: "missing planName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServicePlanExternalName = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "invalid planName",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServicePlanExternalName = "9651.JVHbebe"
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
			name: "in-progress provision with missing InProgressProperties",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress update with missing InProgressProperties",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationUpdate
				i.Status.InProgressProperties = nil
				return i
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present InProgressProperties",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.InProgressProperties = validServiceInstancePropertiesState()
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress deprovision with present InProgressProperties",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.CurrentOperation = servicecatalog.ServiceInstanceOperationDeprovision
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties with no external plan name",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.ClusterServicePlanExternalName = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "in-progress properties with no external plan ID",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.InProgressProperties.ClusterServicePlanExternalID = ""
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
			name: "external properties with no external plan name",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.ClusterServicePlanExternalName = ""
				return i
			}(),
			valid: false,
		},
		{
			name: "external properties with no external plan ID",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.ExternalProperties = validServiceInstancePropertiesState()
				i.Status.ExternalProperties.ClusterServicePlanExternalID = ""
				return i
			}(),
			valid: false,
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
			name: "valid create with k8s name",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceForCreate()
				i.Spec.ClusterServiceClassExternalName = ""
				i.Spec.ClusterServicePlanExternalName = ""
				i.Spec.ClusterServiceClassName = clusterServiceClassName
				i.Spec.ClusterServicePlanName = clusterServicePlanName
				return i
			}(),
			create: true,
			valid:  true,
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
		{
			name: "external and k8s name specified in Spec.PlanReference",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Spec.ClusterServiceClassName = "can not have this here"
				return i
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "failed provision starting orphan mitigation",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.OperationStartTime = nil
				i.Status.OrphanMitigationInProgress = true
				i.Status.Conditions = []servicecatalog.ServiceInstanceCondition{
					{
						Type:   servicecatalog.ServiceInstanceConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
					{
						Type:   servicecatalog.ServiceInstanceConditionFailed,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return i
			}(),
			valid: true,
		},
		{
			name: "in-progress orphan mitigation",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceWithInProgressProvision()
				i.Status.OrphanMitigationInProgress = true
				i.Status.Conditions = []servicecatalog.ServiceInstanceCondition{
					{
						Type:   servicecatalog.ServiceInstanceConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
					{
						Type:   servicecatalog.ServiceInstanceConditionFailed,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return i
			}(),
			valid: true,
		},
		{
			name: "required deprovision status on create",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceForCreate()
				i.Status.DeprovisionStatus = servicecatalog.ServiceInstanceDeprovisionStatusRequired
				return i
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "succeeded deprovision status on create",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceForCreate()
				i.Status.DeprovisionStatus = servicecatalog.ServiceInstanceDeprovisionStatusSucceeded
				return i
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "failed deprovision status on create",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstanceForCreate()
				i.Status.DeprovisionStatus = servicecatalog.ServiceInstanceDeprovisionStatusFailed
				return i
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "invalid deprovision status on update",
			instance: func() *servicecatalog.ServiceInstance {
				i := validServiceInstance()
				i.Status.DeprovisionStatus = servicecatalog.ServiceInstanceDeprovisionStatus("bad-deprovision-status")
				return i
			}(),
			valid: false,
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
					ClusterServiceClassExternalName: clusterServiceClassExternalName,
					ClusterServicePlanExternalName:  clusterServicePlanExternalName,
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
					ClusterServiceClassExternalName: "test-serviceclass",
					ClusterServicePlanExternalName:  "test-plan",
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
		newPlanRef *servicecatalog.ClusterObjectReference
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
			newPlanRef: &servicecatalog.ClusterObjectReference{},
			valid:      false,
		},
		{
			name:       "no plan change",
			oldPlan:    "plan-name",
			newPlan:    "plan-name",
			newPlanRef: &servicecatalog.ClusterObjectReference{},
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
					ClusterServiceClassExternalName: "test-serviceclass",
					ClusterServicePlanExternalName:  tc.oldPlan,
				},
				ClusterServiceClassRef: &servicecatalog.ClusterObjectReference{},
				ClusterServicePlanRef:  &servicecatalog.ClusterObjectReference{},
			},
		}

		newInstance := &servicecatalog.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-instance",
				Namespace: "test-ns",
			},
			Spec: servicecatalog.ServiceInstanceSpec{
				PlanReference: servicecatalog.PlanReference{
					ClusterServiceClassExternalName: clusterServiceClassExternalName,
					ClusterServicePlanExternalName:  tc.newPlan,
				},
				ClusterServiceClassRef: &servicecatalog.ClusterObjectReference{},
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
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: validServiceInstancePropertiesState(),
				AsyncOpInProgress:    true,
				DeprovisionStatus:    servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			valid: true,
			err:   "",
		},
		{
			name: "Complete async op",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: validServiceInstancePropertiesState(),
				AsyncOpInProgress:    true,
				DeprovisionStatus:    servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				AsyncOpInProgress: false,
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
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
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: validServiceInstancePropertiesState(),
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			valid: false,
			err:   "operation in progress",
		},
		{
			name: "ServiceInstanceConditionReady can be true if operation is completed",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: validServiceInstancePropertiesState(),
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionFalse,
				}},
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation: "",
				Conditions: []servicecatalog.ServiceInstanceCondition{{
					Type:   servicecatalog.ServiceInstanceConditionReady,
					Status: servicecatalog.ConditionTrue,
				}},
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update non-ready instance condition during operation",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: validServiceInstancePropertiesState(),
				Conditions:           []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
				DeprovisionStatus:    servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:     servicecatalog.ServiceInstanceOperationProvision,
				OperationStartTime:   &now,
				InProgressProperties: validServiceInstancePropertiesState(),
				Conditions:           []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
				DeprovisionStatus:    servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			valid: true,
			err:   "",
		},
		{
			name: "Update non-ready instance condition outside of operation",
			old: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:  "",
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionFalse}},
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:  "",
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
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
				DeprovisionStatus:    servicecatalog.ServiceInstanceDeprovisionStatusRequired,
			},
			new: &servicecatalog.ServiceInstanceStatus{
				CurrentOperation:  "",
				Conditions:        []servicecatalog.ServiceInstanceCondition{{Status: servicecatalog.ConditionTrue}},
				DeprovisionStatus: servicecatalog.ServiceInstanceDeprovisionStatusRequired,
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
					ClusterServiceClassExternalName: clusterServiceClassExternalName,
					ClusterServicePlanExternalName:  clusterServicePlanExternalName,
				},
				ClusterServiceClassRef: &servicecatalog.ClusterObjectReference{},
				ClusterServicePlanRef:  &servicecatalog.ClusterObjectReference{},
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
					ClusterServiceClassExternalName: clusterServiceClassExternalName,
					ClusterServicePlanExternalName:  clusterServicePlanExternalName,
				},
				ClusterServiceClassRef: &servicecatalog.ClusterObjectReference{},
				ClusterServicePlanRef:  &servicecatalog.ClusterObjectReference{},
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
				i.Spec.ClusterServiceClassRef = &servicecatalog.ClusterObjectReference{
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
				i.Spec.ClusterServicePlanRef = &servicecatalog.ClusterObjectReference{
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
				i.Spec.ClusterServicePlanRef = &servicecatalog.ClusterObjectReference{
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

func TestValidatePlanReference(t *testing.T) {
	cases := []struct {
		name          string
		ref           servicecatalog.PlanReference
		valid         bool
		expectedError string
	}{
		{
			name:          "invalid -- empty struct",
			ref:           servicecatalog.PlanReference{},
			valid:         false,
			expectedError: "exactly one of clusterServicePlanExternalName",
		},
		{
			name:  "valid -- external",
			ref:   validPlanReferenceExternal(),
			valid: true,
		},
		{
			name: "invalid -- external name, k8s plan",
			ref: servicecatalog.PlanReference{
				ClusterServiceClassExternalName: clusterServiceClassExternalName,
				ClusterServicePlanName:          clusterServicePlanExternalName,
			},
			valid:         false,
			expectedError: "must specify clusterServicePlanExternalName",
		},
		{
			name:  "valid -- k8s",
			ref:   validPlanReferenceK8S(),
			valid: true,
		},
		{
			name: "invalid -- valid k8s name, external plan",
			ref: servicecatalog.PlanReference{
				ClusterServiceClassName:        clusterServiceClassName,
				ClusterServicePlanExternalName: clusterServicePlanExternalName,
			},
			valid:         false,
			expectedError: "must specify clusterServicePlanName",
		},
		{
			name: "invalid -- external and k8s name specified",
			ref: servicecatalog.PlanReference{
				ClusterServiceClassName:         clusterServiceClassName,
				ClusterServiceClassExternalName: clusterServiceClassExternalName,
				ClusterServicePlanExternalName:  clusterServicePlanExternalName,
			},
			valid:         false,
			expectedError: "exactly one of clusterServiceClassExternalName",
		},
		{
			name: "invalid -- external and k8s plan specified",
			ref: servicecatalog.PlanReference{
				ClusterServiceClassExternalName: clusterServiceClassExternalName,
				ClusterServicePlanExternalName:  clusterServicePlanExternalName,
				ClusterServicePlanName:          clusterServicePlanName,
			},
			valid:         false,
			expectedError: "exactly one of clusterServicePlanExternalName",
		},
	}
	for _, tc := range cases {
		errs := validatePlanReference(&tc.ref, field.NewPath("spec"))
		if len(errs) != 0 {
			if tc.valid {
				t.Errorf("%v: unexpected error: %v", tc.name, errs)
				continue
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), tc.expectedError) {
					found = true
				}
			}
			if !found {
				t.Errorf("%v: did not find expected error %q in errors: %v", tc.name, tc.expectedError, errs)
				continue
			}
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}

func TestValidatePlanReferenceUpdate(t *testing.T) {
	cases := []struct {
		name          string
		old           servicecatalog.PlanReference
		new           servicecatalog.PlanReference
		valid         bool
		expectedError string
	}{
		{
			name:  "valid -- no changes external",
			old:   validPlanReferenceExternal(),
			new:   validPlanReferenceExternal(),
			valid: true,
		},
		{
			name:  "valid -- no changes k8s",
			old:   validPlanReferenceK8S(),
			new:   validPlanReferenceK8S(),
			valid: true,
		},
		{
			name: "invalid -- changing external class name",
			old:  validPlanReferenceExternal(),
			new: servicecatalog.PlanReference{
				ClusterServiceClassExternalName: clusterServiceClassName,
				ClusterServicePlanExternalName:  clusterServicePlanExternalName,
			},
			valid:         false,
			expectedError: "clusterServiceClassExternalName",
		},
		{
			name: "valid -- changing external plan name",
			old:  validPlanReferenceExternal(),
			new: servicecatalog.PlanReference{
				ClusterServiceClassExternalName: clusterServiceClassExternalName,
				ClusterServicePlanExternalName:  clusterServicePlanName,
			},
			valid: true,
		},
		{
			name: "invalid -- changing k8s class name",
			old:  validPlanReferenceK8S(),
			new: servicecatalog.PlanReference{
				ClusterServiceClassName: clusterServiceClassExternalName,
				ClusterServicePlanName:  clusterServicePlanName,
			},
			valid:         false,
			expectedError: "clusterServiceClassName",
		},
		{
			name: "alid -- changing k8s plan name",
			old:  validPlanReferenceK8S(),
			new: servicecatalog.PlanReference{
				ClusterServiceClassName: clusterServiceClassName,
				ClusterServicePlanName:  clusterServicePlanExternalName,
			},
			valid: true,
		},
	}
	for _, tc := range cases {
		errs := validatePlanReferenceUpdate(&tc.old, &tc.new, field.NewPath("spec"))
		if len(errs) != 0 {
			if tc.valid {
				t.Errorf("%v: unexpected error: %v", tc.name, errs)
				continue
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), tc.expectedError) {
					found = true
				}
			}
			if !found {
				t.Errorf("%v: did not find expected error %q in errors: %v", tc.name, tc.expectedError, errs)
				continue
			}
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
