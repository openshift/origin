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

package integration

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"

	// avoid error `servicecatalog/v1beta1 is not enabled`
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"github.com/kubernetes-incubator/service-catalog/test/util"
)

// TestCreateServiceBindingSuccess successful paths binding
func TestCreateServiceBindingSuccess(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "defaults",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:        t,
				broker:   getTestBroker(),
				instance: getTestInstance(),
				binding:  getTestBinding(),
			}
			ct.run(func(ct *controllerTest) {
				condition := v1beta1.ServiceBindingCondition{
					Type:   v1beta1.ServiceBindingConditionReady,
					Status: v1beta1.ConditionTrue,
				}
				if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, condition); err != nil {
					t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, condition, cond)
				}
			})
		})
	}
}

// TestCreateServiceBindingInvalidInstanceFailure try to bind to invalid service instance names
func TestCreateServiceBindingInvalidInstanceFailure(t *testing.T) {
	cases := []struct {
		name         string
		instanceName *string
	}{
		{
			name:         "invalid service instance name",
			instanceName: strPtr(""),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:        t,
				broker:   getTestBroker(),
				instance: getTestInstance(),
			}
			ct.run(func(ct *controllerTest) {
				binding := getTestBinding()
				if tc.instanceName != nil {
					binding.Spec.ServiceInstanceRef.Name = *tc.instanceName
				}

				if _, err := ct.client.ServiceBindings(binding.Namespace).Create(binding); err == nil {
					t.Fatalf("expected binding to fail to be created due to invalid parameters")
				}
			})
		})
	}
}

// TestCreateServiceBindingInvalidInstance try to bind to invalid service instance names
func TestCreateServiceBindingInvalidInstance(t *testing.T) {
	cases := []struct {
		name         string
		instanceName *string
		condition    v1beta1.ServiceBindingCondition
	}{
		{
			name:         "non-existent service instance name",
			instanceName: strPtr("nothereinstance"),

			condition: v1beta1.ServiceBindingCondition{
				Type:   v1beta1.ServiceBindingConditionReady,
				Status: v1beta1.ConditionFalse,
				Reason: "ReferencesNonexistentInstance",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:        t,
				broker:   getTestBroker(),
				instance: getTestInstance(),
				binding: func() *v1beta1.ServiceBinding {
					b := getTestBinding()
					if tc.instanceName != nil {
						b.Spec.ServiceInstanceRef.Name = *tc.instanceName
					}
					return b
				}(),
				skipVerifyingBindingSuccess: true,
			}
			ct.run(func(ct *controllerTest) {
				if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, tc.condition); err != nil {
					t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, tc.condition, cond)
				}
			})
		})
	}
}

// TestCreateServiceBindingNonBindable bind to a non-bindable service class / plan.
func TestCreateServiceBindingNonBindable(t *testing.T) {
	cases := []struct {
		name            string
		nonbindablePlan bool
		condition       v1beta1.ServiceBindingCondition
	}{
		{
			name:            "non-bindable plan",
			nonbindablePlan: true,
			condition: v1beta1.ServiceBindingCondition{
				Type:   v1beta1.ServiceBindingConditionReady,
				Status: v1beta1.ConditionFalse,
				Reason: "ErrorNonbindableServiceClass",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:      t,
				broker: getTestBroker(),
				instance: func() *v1beta1.ServiceInstance {
					i := getTestInstance()
					if tc.nonbindablePlan {
						i.Spec.PlanReference.ClusterServicePlanExternalName = testNonbindableClusterServicePlanName
					}
					return i
				}(),
				binding:                     getTestBinding(),
				skipVerifyingBindingSuccess: true,
			}
			ct.run(func(ct *controllerTest) {
				if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, tc.condition); err != nil {
					t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, tc.condition, cond)
				}
			})
		})
	}
}

// TestCreateServiceBindingInstanceNotReady bind to a service instance in the ready false state.
func TestCreateServiceBindingInstanceNotReady(t *testing.T) {
	cases := []struct {
		name             string
		instanceNotReady bool
		condition        v1beta1.ServiceBindingCondition
	}{
		{
			name:             "service instance not ready",
			instanceNotReady: true,
			condition: v1beta1.ServiceBindingCondition{
				Type:   v1beta1.ServiceBindingConditionReady,
				Status: v1beta1.ConditionFalse,
				Reason: "ErrorInstanceNotReady",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &controllerTest{
				t:        t,
				broker:   getTestBroker(),
				instance: getTestInstance(),
				binding:  getTestBinding(),
				setup: func(ct *controllerTest) {
					if tc.instanceNotReady {
						reactionError := osb.HTTPStatusCodeError{
							StatusCode:   http.StatusBadGateway,
							ErrorMessage: strPtr("error message"),
							Description:  strPtr("response description"),
						}
						ct.osbClient.ProvisionReaction = &fakeosb.ProvisionReaction{
							Error: reactionError,
						}
						ct.skipVerifyingInstanceSuccess = true
					}
				},
				skipVerifyingBindingSuccess: true,
			}
			ct.run(func(ct *controllerTest) {
				if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, tc.condition); err != nil {
					t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, tc.condition, cond)
				}
			})
		})
	}
}

// TestCreateServiceBindingWithParameters tests creating a ServiceBinding
// with parameters.
func TestCreateServiceBindingWithParameters(t *testing.T) {
	type secretDef struct {
		name string
		data map[string][]byte
	}
	cases := []struct {
		name           string
		params         map[string]interface{}
		paramsFrom     []v1beta1.ParametersFromSource
		secrets        []secretDef
		expectedParams map[string]interface{}
		expectedError  bool
	}{
		{
			name:           "no params",
			expectedParams: nil,
		},
		{
			name: "plain params",
			params: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			},
			expectedParams: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			},
		},
		{
			name: "secret params",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`{"A":"B","C":{"D":"E","F":"G"}}`),
					},
				},
			},
			expectedParams: map[string]interface{}{
				"A": "B",
				"C": map[string]interface{}{
					"D": "E",
					"F": "G",
				},
			},
		},
		{
			name: "plain and secret params",
			params: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
			},
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`{"A":"B","C":{"D":"E","F":"G"}}`),
					},
				},
			},
			expectedParams: map[string]interface{}{
				"Name": "test-param",
				"Args": map[string]interface{}{
					"first":  "first-arg",
					"second": "second-arg",
				},
				"A": "B",
				"C": map[string]interface{}{
					"D": "E",
					"F": "G",
				},
			},
		},
		{
			name: "missing secret",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			expectedError: true,
		},
		{
			name: "missing secret key",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "other-secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`bad`),
					},
				},
			},
			expectedError: true,
		},
		{
			name: "empty secret data",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{},
				},
			},
			expectedError: true,
		},
		{
			name: "bad secret data",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`bad`),
					},
				},
			},
			expectedError: true,
		},
		{
			name: "no params in secret data",
			paramsFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret-name",
						Key:  "secret-key",
					},
				},
			},
			secrets: []secretDef{
				{
					name: "secret-name",
					data: map[string][]byte{
						"secret-key": []byte(`{}`),
					},
				},
			},
			expectedParams: nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			//t.Parallel()
			ct := &controllerTest{
				t:        t,
				broker:   getTestBroker(),
				instance: getTestInstance(),
				binding: func() *v1beta1.ServiceBinding {
					b := getTestBinding()
					if tc.params != nil {
						b.Spec.Parameters = convertParametersIntoRawExtension(t, tc.params)
					}
					b.Spec.ParametersFrom = tc.paramsFrom
					return b
				}(),
				skipVerifyingBindingSuccess: tc.expectedError,
				setup: func(ct *controllerTest) {
					for _, secret := range tc.secrets {
						prependGetSecretReaction(ct.kubeClient, secret.name, secret.data)
					}
				},
			}
			ct.run(func(ct *controllerTest) {
				if tc.expectedError {
					condition := v1beta1.ServiceBindingCondition{
						Type:   v1beta1.ServiceBindingConditionReady,
						Status: v1beta1.ConditionFalse,
						Reason: "ErrorWithParameters",
					}
					if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, condition); err != nil {
						t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, condition, cond)
					}
				} else {
					brokerAction := getLastBrokerAction(t, ct.osbClient, fakeosb.Bind)
					if e, a := tc.expectedParams, brokerAction.Request.(*osb.BindRequest).Parameters; !reflect.DeepEqual(e, a) {
						t.Fatalf("unexpected diff in provision parameters: expected %v, got %v", e, a)
					}
				}
			})
		})
	}
}

// TestDeleteServiceBindingRetry tests whether deletion of a service binding
// retries after failing.
func TestDeleteServiceBindingFailureRetry(t *testing.T) {
	const NumberOfUnbindFailures = 2
	numberOfAttempts := 0
	ct := &controllerTest{
		t:        t,
		broker:   getTestBroker(),
		instance: getTestInstance(),
		binding:  getTestBinding(),
		setup: func(ct *controllerTest) {
			ct.osbClient.UnbindReaction = fakeosb.DynamicUnbindReaction(
				func(_ *osb.UnbindRequest) (*osb.UnbindResponse, error) {
					numberOfAttempts++
					if numberOfAttempts > NumberOfUnbindFailures {
						return &osb.UnbindResponse{}, nil
					}
					return nil, osb.HTTPStatusCodeError{
						StatusCode:  500,
						Description: strPtr("test error unbinding"),
					}
				})
		},
	}
	ct.run(func(_ *controllerTest) {})
}

// TestDeleteServiceBindingRetry tests whether deletion of a service binding
// retries after failing an asynchronous unbind.
func TestDeleteServiceBindingFailureRetryAsync(t *testing.T) {
	// Enable the AsyncBindingOperations feature
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	hasPollFailed := false
	ct := &controllerTest{
		t:        t,
		broker:   getTestBroker(),
		instance: getTestInstance(),
		binding:  getTestBinding(),
		setup: func(ct *controllerTest) {
			ct.osbClient.UnbindReaction = fakeosb.DynamicUnbindReaction(
				func(_ *osb.UnbindRequest) (*osb.UnbindResponse, error) {
					response := &osb.UnbindResponse{Async: true}
					if hasPollFailed {
						response.Async = false
					}
					return response, nil
				})

			ct.osbClient.PollBindingLastOperationReaction = fakeosb.DynamicPollBindingLastOperationReaction(
				func(_ *osb.BindingLastOperationRequest) (*osb.LastOperationResponse, error) {
					hasPollFailed = true
					return &osb.LastOperationResponse{
						State: osb.StateFailed,
					}, nil
				})
		},
	}
	ct.run(func(_ *controllerTest) {})
}
