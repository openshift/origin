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
	"k8s.io/apimachinery/pkg/runtime"
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

func validServiceInstanceCredentialWithInProgressBind() *servicecatalog.ServiceInstanceCredential {
	binding := validServiceInstanceCredential()
	binding.Generation = 2
	binding.Status.ReconciledGeneration = 1
	binding.Status.CurrentOperation = servicecatalog.ServiceInstanceCredentialOperationBind
	now := metav1.Now()
	binding.Status.OperationStartTime = &now
	binding.Status.InProgressProperties = validServiceInstanceCredentialPropertiesState()
	return binding
}

func validServiceInstanceCredentialPropertiesState() *servicecatalog.ServiceInstanceCredentialPropertiesState {
	return &servicecatalog.ServiceInstanceCredentialPropertiesState{
		Parameters:         &runtime.RawExtension{Raw: []byte("a: 1\nb: \"2\"")},
		ParametersChecksum: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

func TestValidateServiceInstanceCredential(t *testing.T) {
	cases := []struct {
		name    string
		binding *servicecatalog.ServiceInstanceCredential
		create  bool
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
		{
			name:    "valid with in-progress bind",
			binding: validServiceInstanceCredentialWithInProgressBind(),
			valid:   true,
		},
		{
			name: "valid with in-progress unbind",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.CurrentOperation = servicecatalog.ServiceInstanceCredentialOperationUnbind
				b.Status.InProgressProperties = nil
				return b
			}(),
			valid: true,
		},
		{
			name: "invalid current operation",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.CurrentOperation = servicecatalog.ServiceInstanceCredentialOperation("bad-operation")
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress without updated generation",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.ReconciledGeneration = b.Generation
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress with missing OperationStartTime",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.OperationStartTime = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present OperationStartTime",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				now := metav1.Now()
				b.Status.OperationStartTime = &now
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress with condition ready/true",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.Conditions = []servicecatalog.ServiceInstanceCredentialCondition{
					{
						Type:   servicecatalog.ServiceInstanceCredentialConditionReady,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress with condition ready/false",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.Conditions = []servicecatalog.ServiceInstanceCredentialCondition{
					{
						Type:   servicecatalog.ServiceInstanceCredentialConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
				}
				return b
			}(),
			valid: true,
		},
		{
			name: "in-progress bind with missing InProgressParameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present InProgressParameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.InProgressProperties = validServiceInstanceCredentialPropertiesState()
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress unbind with present InProgressParameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.CurrentOperation = servicecatalog.ServiceInstanceCredentialOperationUnbind
				return b
			}(),
			valid: false,
		},
		{
			name: "valid in-progress properties with no parameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.Parameters = nil
				b.Status.InProgressProperties.ParametersChecksum = ""
				return b
			}(),
			valid: true,
		},
		{
			name: "in-progress properties parameters with missing parameters checksum",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.ParametersChecksum = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum with missing parameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.Parameters = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters with missing raw",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.Parameters.Raw = []byte{}
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters with malformed yaml",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.Parameters.Raw = []byte("bad yaml")
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum too small",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.ParametersChecksum = "0123456"
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum malformed",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredentialWithInProgressBind()
				b.Status.InProgressProperties.ParametersChecksum = "not hex"
				return b
			}(),
			valid: false,
		},
		{
			name: "valid external properties",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				return b
			}(),
			valid: true,
		},
		{
			name: "valid external properties with no parameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.Parameters = nil
				b.Status.ExternalProperties.ParametersChecksum = ""
				return b
			}(),
			valid: true,
		},
		{
			name: "external properties parameters with missing parameters checksum",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.ParametersChecksum = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum with missing parameters",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.Parameters = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters with missing raw",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.Parameters.Raw = []byte{}
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters with malformed yaml",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.Parameters.Raw = []byte("bad yaml")
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum too small",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.ParametersChecksum = "0123456"
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum malformed",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Status.ExternalProperties = validServiceInstanceCredentialPropertiesState()
				b.Status.ExternalProperties.ParametersChecksum = "not hex"
				return b
			}(),
			valid: false,
		},
		{
			name: "valid create",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Generation = 1
				b.Status.ReconciledGeneration = 0
				return b
			}(),
			create: true,
			valid:  true,
		},
		{
			name: "create with operation in-progress",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Generation = 1
				b.Status.ReconciledGeneration = 0
				b.Status.CurrentOperation = servicecatalog.ServiceInstanceCredentialOperationBind
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "create with invalid reconciled generation",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Generation = 1
				b.Status.ReconciledGeneration = 1
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "update with invalid reconciled generation",
			binding: func() *servicecatalog.ServiceInstanceCredential {
				b := validServiceInstanceCredential()
				b.Generation = 1
				b.Status.ReconciledGeneration = 2
				return b
			}(),
			create: false,
			valid:  false,
		},
	}

	for _, tc := range cases {
		errs := internalValidateServiceInstanceCredential(tc.binding, tc.create)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}

func TestInternalValidateServiceInstanceCredentialUpdateAllowed(t *testing.T) {
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
		oldBinding := validServiceInstanceCredential()
		if tc.onGoingSpecChange {
			oldBinding.Generation = 2
		} else {
			oldBinding.Generation = 1
		}
		oldBinding.Status.ReconciledGeneration = 1

		newBinding := validServiceInstanceCredential()
		if tc.newSpecChange {
			newBinding.Generation = oldBinding.Generation + 1
		} else {
			newBinding.Generation = oldBinding.Generation
		}
		newBinding.Status.ReconciledGeneration = 1

		errs := internalValidateServiceInstanceCredentialUpdateAllowed(newBinding, oldBinding)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
