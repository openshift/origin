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
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	"github.com/golang/glog"
	// avoid error `servicecatalog/v1beta1 is not enabled`
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	clientsetsc "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	scinformers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/controller"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/kubernetes-incubator/service-catalog/test/util"
)

const (
	testNamespace                         = "test-namespace"
	testClusterServiceBrokerName          = "test-broker"
	testClusterServiceClassName           = "test-service"
	testClusterServiceClassGUID           = "12345"
	testClusterServicePlanName            = "test-plan"
	testNonbindableClusterServicePlanName = "test-nb-plan"
	testInstanceLastOperation             = "InstanceLastOperation"
	testPlanExternalID                    = "34567"
	testNonbindablePlanExternalID         = "nb34567"
	testInstanceName                      = "test-instance"
	testBindingName                       = "test-binding"
	testSecretName                        = "test-secret"
	testBrokerURL                         = "https://example.com"
	testExternalID                        = "9737b6ed-ca95-4439-8219-c53fcad118ab"
	testDashboardURL                      = "http://test-dashboard.example.com"
	testOperation                         = "test-operation"
)

// TestBasicFlows tests:
//
// - add Broker
// - verify ClusterServiceClasses added
// - provision Instance
// - update Instance
// - make Binding
// - unbind
// - deprovision
// - delete broker
func TestBasicFlows(t *testing.T) {
	cases := []struct {
		name              string
		asyncForInstances bool
		asyncForBindings  bool
	}{
		{
			name: "sync",
		},
		{
			name:              "async instances",
			asyncForInstances: true,
		},
		{
			name:             "async bindings",
			asyncForBindings: true,
		},
		{
			name:              "async instances and bindings",
			asyncForInstances: true,
			asyncForBindings:  true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.asyncForBindings {
				// Enable the AsyncBindingOperations feature
				utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
				defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))
			}

			ct := &controllerTest{
				t:        t,
				broker:   getTestBroker(),
				instance: getTestInstance(),
				binding:  getTestBinding(),
				setup: func(ct *controllerTest) {
					if tc.asyncForInstances {
						ct.osbClient.ProvisionReaction.(*fakeosb.ProvisionReaction).Response.Async = true
						ct.osbClient.UpdateInstanceReaction.(*fakeosb.UpdateInstanceReaction).Response.Async = true
						ct.osbClient.DeprovisionReaction.(*fakeosb.DeprovisionReaction).Response.Async = true
					}

					if tc.asyncForBindings {
						ct.osbClient.BindReaction.(*fakeosb.BindReaction).Response.Async = true
						ct.osbClient.UnbindReaction.(*fakeosb.UnbindReaction).Response.Async = true
					}
				},
			}
			ct.run(func(ct *controllerTest) {
				// Update instance
				updateRequests := ct.instance.Spec.UpdateRequests + 1
				ct.instance.Spec.UpdateRequests = updateRequests
				if _, err := ct.client.ServiceInstances(testNamespace).Update(ct.instance); err != nil {
					t.Fatalf("error updating Instance: %v", err)
				}

				if err := util.WaitForInstanceReconciledGeneration(ct.client, testNamespace, testInstanceName, ct.instance.Status.ReconciledGeneration+1); err != nil {
					t.Fatalf("error waiting for instance to reconcile: %v", err)
				}

				retInst, err := ct.client.ServiceInstances(ct.instance.Namespace).Get(ct.instance.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("error getting instance %s/%s back", ct.instance.Namespace, ct.instance.Name)
				}
				if e, a := updateRequests, retInst.Spec.UpdateRequests; e != a {
					t.Fatalf("unexpected updateRequets in instance spec: expected %v, got %v", e, a)
				}
			})
		})
	}
}

// TestBindingFailure tests that a binding gets a failure condition when the
// broker returns a failure response for a bind operation.
func TestBindingFailure(t *testing.T) {
	ct := &controllerTest{
		t:                           t,
		broker:                      getTestBroker(),
		instance:                    getTestInstance(),
		binding:                     getTestBinding(),
		skipVerifyingBindingSuccess: true,
		setup: func(ct *controllerTest) {
			ct.osbClient.BindReaction = &fakeosb.BindReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode:   http.StatusConflict,
					ErrorMessage: strPtr("ServiceBindingExists"),
					Description:  strPtr("Service binding with the same id, for the same service instance already exists."),
				},
			}
		},
	}
	ct.run(func(ct *controllerTest) {
		condition := v1beta1.ServiceBindingCondition{
			Type:   v1beta1.ServiceBindingConditionFailed,
			Status: v1beta1.ConditionTrue,
		}
		if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, condition); err != nil {
			t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, condition, cond)
		}
	})
}

// verifyUsernameInLastBrokerAction verifies that the originating identity sent in the request to the broker
// included the specified username.
func verifyUsernameInLastBrokerAction(t *testing.T, osbClient *fakeosb.FakeClient, actionType fakeosb.ActionType, username string) {
	brokerAction := getLastBrokerAction(t, osbClient, actionType)
	var oi *osb.OriginatingIdentity
	switch request := brokerAction.Request.(type) {
	case *osb.ProvisionRequest:
		oi = request.OriginatingIdentity
	case *osb.UpdateInstanceRequest:
		oi = request.OriginatingIdentity
	case *osb.DeprovisionRequest:
		oi = request.OriginatingIdentity
	case *osb.BindRequest:
		oi = request.OriginatingIdentity
	case *osb.UnbindRequest:
		oi = request.OriginatingIdentity
	case *osb.LastOperationRequest:
		oi = request.OriginatingIdentity
	default:
		t.Fatalf("unexpected request type: %T", request)
	}
	if oi == nil {
		t.Fatalf("originating identity not sent in request")
	}
	if e, a := "kubernetes", oi.Platform; e != a {
		t.Fatalf("unexpected originating identity platform: expected %q, got %q", e, a)
	}
	oiValues := make(map[string]interface{})
	if err := json.Unmarshal([]byte(oi.Value), &oiValues); err != nil {
		t.Fatalf("could not unmarshal originating identity value as json: %v", err)
	}
	if e, a := username, oiValues["username"]; e != a {
		t.Fatalf("unexpected username in originating identity: expected %q, got %q", e, a)
	}
}

// TestBasicFlowsWithOriginatingIdentity tests that the correct username is sent
// as part of originating identity included in broker requests.
func TestBasicFlowsWithOriginatingIdentity(t *testing.T) {
	// Enable the OriginatingIdentity feature
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))

	createChangeUsernameFunc := func(username string) func(*controllerTest) {
		return func(ct *controllerTest) {
			catalogClient, err := changeUsernameForCatalogClient(ct.catalogClient, ct.catalogClientConfig, username)
			if err != nil {
				t.Fatalf("could not change the username for the catalog client: %v", err)
			}
			ct.catalogClient = catalogClient
			ct.client = catalogClient.ServicecatalogV1beta1()
		}
	}

	createVerifyUsernameFunc := func(actionType fakeosb.ActionType, username string) func(*controllerTest) {
		return func(ct *controllerTest) {
			verifyUsernameInLastBrokerAction(ct.t, ct.osbClient, actionType, username)
		}
	}

	ct := &controllerTest{
		t:                  t,
		broker:             getTestBroker(),
		instance:           getTestInstance(),
		binding:            getTestBinding(),
		preCreateInstance:  createChangeUsernameFunc("instance-creator"),
		postCreateInstance: createVerifyUsernameFunc(fakeosb.ProvisionInstance, "instance-creator"),
		preDeleteInstance:  createChangeUsernameFunc("instance-deleter"),
		postDeleteInstance: createVerifyUsernameFunc(fakeosb.DeprovisionInstance, "instance-deleter"),
		preCreateBinding:   createChangeUsernameFunc("binding-creator"),
		postCreateBinding:  createVerifyUsernameFunc(fakeosb.Bind, "binding-creator"),
		preDeleteBinding:   createChangeUsernameFunc("binding-deleter"),
		postDeleteBinding:  createVerifyUsernameFunc(fakeosb.Unbind, "binding-deleter"),
	}
	ct.run(func(ct *controllerTest) {
		// Update Instance
		createChangeUsernameFunc("instance-updater")(ct)

		ct.instance.Spec.UpdateRequests = ct.instance.Spec.UpdateRequests + 1
		if _, err := ct.client.ServiceInstances(testNamespace).Update(ct.instance); err != nil {
			t.Fatalf("error updating Instance: %v", err)
		}

		if err := util.WaitForInstanceReconciledGeneration(ct.client, testNamespace, testInstanceName, ct.instance.Status.ReconciledGeneration+1); err != nil {
			t.Fatalf("error waiting for instance to reconcile: %v", err)
		}

		verifyUsernameInLastBrokerAction(t, ct.osbClient, fakeosb.UpdateInstance, "instance-updater")
	})
}

// TestServiceBindingOrphanMitigation tests whether a binding is
// successfully deleted after a bind request returns a status code
// that should trigger orphan mitigation.
func TestServiceBindingOrphanMitigation(t *testing.T) {
	ct := &controllerTest{
		t:                           t,
		broker:                      getTestBroker(),
		instance:                    getTestInstance(),
		binding:                     getTestBinding(),
		skipVerifyingBindingSuccess: true,
		setup: func(ct *controllerTest) {
			ct.osbClient.BindReaction = &fakeosb.BindReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusInternalServerError,
				},
			}
		},
	}
	ct.run(func(ct *controllerTest) {
		condition := v1beta1.ServiceBindingCondition{
			Type:   v1beta1.ServiceBindingConditionFailed,
			Status: v1beta1.ConditionTrue,
		}
		if cond, err := util.WaitForBindingCondition(ct.client, testNamespace, testBindingName, condition); err != nil {
			t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, condition, cond)
		}

		if err := util.WaitForBindingReconciledGeneration(ct.client, testNamespace, testBindingName, 1); err != nil {
			t.Fatalf("error waiting for binding to reconcile: %v", err)
		}

		retBinding, err := ct.client.ServiceBindings(testNamespace).Get(testBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("error getting binding %s/%s back", testNamespace, testBindingName)
		}

		util.AssertServiceBindingCondition(
			t,
			retBinding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			"OrphanMitigationSuccessful",
		)
	})
}

// TestAsyncProvisionWithMultiplePolls tests an async instance provisioning
// where the first few last-operation polls respond still in-progress.
func TestAsyncProvisionWithMultiplePolls(t *testing.T) {
	const NumberOfInProgressResponses = 2
	numberOfPolls := 0

	ct := &controllerTest{
		t:                            t,
		broker:                       getTestBroker(),
		instance:                     getTestInstance(),
		skipVerifyingInstanceSuccess: true,
		setup: func(ct *controllerTest) {
			ct.osbClient.ProvisionReaction.(*fakeosb.ProvisionReaction).Response.Async = true
			ct.osbClient.PollLastOperationReaction = fakeosb.DynamicPollLastOperationReaction(
				func(_ *osb.LastOperationRequest) (*osb.LastOperationResponse, error) {
					numberOfPolls++
					state := osb.StateInProgress
					if numberOfPolls > NumberOfInProgressResponses {
						state = osb.StateSucceeded
					}
					return &osb.LastOperationResponse{State: state}, nil
				})
		},
	}
	ct.run(func(ct *controllerTest) {
		// Polling is going to take at least 3 seconds, with a 1-second break between the first poll and the second
		// and a 2-second break between the second poll and the third. Let's sleep here so that we don't risk
		// timing out while waiting for the instance condition to be ready in the following wait.
		//time.Sleep(3 * time.Second)

		verifyInstanceCreated(t, ct.client, ct.instance)
	})
}

// TestServiceInstanceDeleteWithAsyncUpdateInProgress tests that you can delete
// an instance during an async update.  That is, if you request a delete during
// an instance update, the instance will be deleted when the update completes
// regardless of success or failure.
func TestServiceInstanceDeleteWithAsyncUpdateInProgress(t *testing.T) {

	cases := []struct {
		name           string
		updateSucceeds bool
	}{
		{
			name:           "update succeeds",
			updateSucceeds: true,
		},
		{
			name:           "update fails",
			updateSucceeds: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var done int32 = 0
			ct := controllerTest{
				t:                            t,
				broker:                       getTestBroker(),
				instance:                     getTestInstance(),
				skipVerifyingInstanceSuccess: false,
				setup: func(ct *controllerTest) {
					ct.osbClient.UpdateInstanceReaction.(*fakeosb.UpdateInstanceReaction).Response.Async = true
					ct.osbClient.PollLastOperationReaction = fakeosb.DynamicPollLastOperationReaction(
						func(_ *osb.LastOperationRequest) (*osb.LastOperationResponse, error) {
							state := osb.StateInProgress
							d := atomic.LoadInt32(&done)
							if d > 0 {
								if tc.updateSucceeds {
									state = osb.StateSucceeded
								} else {
									state = osb.StateFailed
								}
							}
							return &osb.LastOperationResponse{State: state}, nil
						})
				},
			}
			ct.run(func(ct *controllerTest) {

				if err := util.WaitForInstanceCondition(ct.client, ct.instance.Namespace, ct.instance.Name,
					v1beta1.ServiceInstanceCondition{
						Type:   v1beta1.ServiceInstanceConditionReady,
						Status: v1beta1.ConditionTrue,
						Reason: "ProvisionedSuccessfully",
					}); err != nil {
					t.Fatalf("error waiting for instance to be ready: %v", err)
				}

				// add a parameter to the instance for the Update()
				ct.instance.Spec.Parameters = convertParametersIntoRawExtension(t,
					map[string]interface{}{
						"param-key": "new-param-value",
					})

				_, err := ct.client.ServiceInstances(testNamespace).Update(ct.instance)
				if err != nil {
					t.Fatalf("error updating instance: %v", err)
				}

				if err := util.WaitForInstanceCondition(ct.client, ct.instance.Namespace, ct.instance.Name,
					v1beta1.ServiceInstanceCondition{
						Type:   v1beta1.ServiceInstanceConditionReady,
						Status: v1beta1.ConditionFalse,
						Reason: "UpdatingInstance",
					}); err != nil {
					t.Fatalf("error waiting for instance to be updating asynchronously: %v", err)
				}

				if err := ct.client.ServiceInstances(ct.instance.Namespace).Delete(ct.instance.Name, &metav1.DeleteOptions{}); err != nil {
					t.Fatalf("failed to delete instance: %v", err)
				}

				// notify the thread handling DynamicPollLastOperationReaction that it can end the async op
				atomic.StoreInt32(&done, 1)

				if err := util.WaitForInstanceToNotExist(ct.client, ct.instance.Namespace, ct.instance.Name); err != nil {
					t.Fatalf("error waiting for instance to not exist: %v", err)
				}

				// We deleted the instance above, clear it so test cleanup doesn't fail
				ct.instance = nil
			})
		})
	}
}

// TestServiceInstanceDeleteWithAsyncProvisionInProgress tests that you can
// delete an instance during an async provision.  Verify the instance is deleted
// when the provisioning completes regardless of success or failure.
func TestServiceInstanceDeleteWithAsyncProvisionInProgress(t *testing.T) {
	cases := []struct {
		name              string
		provisionSucceeds bool
	}{
		{
			name:              "provision succeeds",
			provisionSucceeds: true,
		},
		{
			name:              "provision fails",
			provisionSucceeds: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var done int32 = 0
			ct := controllerTest{
				t:                            t,
				broker:                       getTestBroker(),
				instance:                     getTestInstance(),
				skipVerifyingInstanceSuccess: true,
				setup: func(ct *controllerTest) {
					ct.osbClient.ProvisionReaction.(*fakeosb.ProvisionReaction).Response.Async = true
					ct.osbClient.PollLastOperationReaction = fakeosb.DynamicPollLastOperationReaction(
						func(_ *osb.LastOperationRequest) (*osb.LastOperationResponse, error) {
							state := osb.StateInProgress
							d := atomic.LoadInt32(&done)
							if d > 0 {
								if tc.provisionSucceeds {
									state = osb.StateSucceeded
								} else {
									state = osb.StateFailed
								}
							}
							return &osb.LastOperationResponse{State: state}, nil
						})
				},
			}
			ct.run(func(ct *controllerTest) {
				if err := util.WaitForInstanceCondition(ct.client, ct.instance.Namespace, ct.instance.Name,
					v1beta1.ServiceInstanceCondition{
						Type:   v1beta1.ServiceInstanceConditionReady,
						Status: v1beta1.ConditionFalse,
						Reason: "Provisioning",
					}); err != nil {
					t.Fatalf("error waiting for instance to be provisioning asynchronously: %v", err)
				}

				if err := ct.client.ServiceInstances(ct.instance.Namespace).Delete(ct.instance.Name, &metav1.DeleteOptions{}); err != nil {
					t.Fatalf("failed to delete instance: %v", err)
				}

				// notify the thread handling DynamicPollLastOperationReaction that it can end the async op
				atomic.StoreInt32(&done, 1)

				if err := util.WaitForInstanceToNotExist(ct.client, ct.instance.Namespace, ct.instance.Name); err != nil {
					t.Fatalf("error waiting for instance to not exist: %v", err)
				}

				// We deleted the instance above, clear it so test cleanup doesn't fail
				ct.instance = nil
			})
		})
	}
}

// TestServiceBindingDeleteWithAsyncBindInProgress tests that you can delete a
// binding during an async bind operation.  Verify the binding is deleted when
// the bind operation completes regardless of success or failure.
func TestServiceBindingDeleteWithAsyncBindInProgress(t *testing.T) {
	cases := []struct {
		name         string
		bindSucceeds bool
	}{
		{
			name:         "bind succeeds",
			bindSucceeds: true,
		},
		{
			name:         "bind fails",
			bindSucceeds: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Enable the AsyncBindingOperations feature
			utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
			defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

			var done int32 = 0
			ct := controllerTest{
				t:                           t,
				broker:                      getTestBroker(),
				instance:                    getTestInstance(),
				binding:                     getTestBinding(),
				skipVerifyingBindingSuccess: true,
				setup: func(ct *controllerTest) {
					ct.osbClient.BindReaction.(*fakeosb.BindReaction).Response.Async = true
					ct.osbClient.PollBindingLastOperationReaction = fakeosb.DynamicPollBindingLastOperationReaction(
						func(_ *osb.BindingLastOperationRequest) (*osb.LastOperationResponse, error) {
							state := osb.StateInProgress
							d := atomic.LoadInt32(&done)
							if d > 0 {
								if tc.bindSucceeds {
									state = osb.StateSucceeded
								} else {
									state = osb.StateFailed
								}
							}
							return &osb.LastOperationResponse{State: state}, nil
						})
				},
			}
			ct.run(func(ct *controllerTest) {
				if _, err := util.WaitForBindingCondition(ct.client, ct.binding.Namespace, ct.binding.Name,
					v1beta1.ServiceBindingCondition{
						Type:   v1beta1.ServiceBindingConditionReady,
						Status: v1beta1.ConditionFalse,
						Reason: "Binding",
					}); err != nil {
					t.Fatalf("error waiting for binding to be created asynchronously: %v", err)
				}

				if err := ct.client.ServiceBindings(ct.binding.Namespace).Delete(ct.binding.Name, &metav1.DeleteOptions{}); err != nil {
					t.Fatalf("failed to delete binding: %v", err)
				}

				// notify the thread handling DynamicPollLastOperationReaction that it can end the async op
				atomic.StoreInt32(&done, 1)

				if err := util.WaitForBindingToNotExist(ct.client, ct.binding.Namespace, ct.binding.Name); err != nil {
					t.Fatalf("error waiting for binding to not exist: %v", err)
				}

				// We deleted the binding above, clear it so test cleanup doesn't fail
				ct.binding = nil
			})
		})
	}
}

func getUpdateInstanceResponseByPollCountReactions(numOfResponses int, stateProgressions []fakeosb.UpdateInstanceReaction) fakeosb.DynamicUpdateInstanceReaction {
	numberOfPolls := 0
	numberOfStates := len(stateProgressions)

	return func(_ *osb.UpdateInstanceRequest) (*osb.UpdateInstanceResponse, error) {
		var reaction fakeosb.UpdateInstanceReaction
		if numberOfPolls > (numOfResponses*numberOfStates)-1 {
			reaction = stateProgressions[numberOfStates-1]
			glog.V(5).Infof("Update instance state progressions done, ended on %v", reaction)
		} else {
			idx := numberOfPolls / numOfResponses
			reaction = stateProgressions[idx]
			glog.V(5).Infof("Update instance state progression on %v (polls:%v, idx:%v)", reaction, numberOfPolls, idx)
		}
		numberOfPolls++
		if reaction.Response != nil {
			return &osb.UpdateInstanceResponse{
				Async:        reaction.Response.Async,
				OperationKey: reaction.Response.OperationKey,
			}, nil
		}
		return nil, reaction.Error
	}
}

func getLastOperationResponseByPollCountReactions(numOfResponses int, stateProgressions []fakeosb.PollLastOperationReaction) fakeosb.DynamicPollLastOperationReaction {
	numberOfPolls := 0
	numberOfStates := len(stateProgressions)

	return func(_ *osb.LastOperationRequest) (*osb.LastOperationResponse, error) {
		var reaction fakeosb.PollLastOperationReaction
		if numberOfPolls > (numOfResponses*numberOfStates)-1 {
			reaction = stateProgressions[numberOfStates-1]
			glog.V(5).Infof("Last operation state progressions done, ended on %v", reaction)
		} else {
			idx := numberOfPolls / numOfResponses
			reaction = stateProgressions[idx]
			glog.V(5).Infof("Last operation state progression on %v (polls:%v, idx:%v)", reaction, numberOfPolls, idx)
		}
		numberOfPolls++
		if reaction.Response != nil {
			return &osb.LastOperationResponse{State: reaction.Response.State}, nil
		}
		return nil, reaction.Error
	}
}

func getLastOperationResponseByPollCountStates(numOfResponses int, stateProgressions []osb.LastOperationState) func(*osb.LastOperationRequest) (*osb.LastOperationResponse, error) {
	reactionProgressions := make([]fakeosb.PollLastOperationReaction, len(stateProgressions))
	for i, item := range stateProgressions {
		newReaction := fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: item,
			},
		}
		reactionProgressions[i] = newReaction
	}

	return getLastOperationResponseByPollCountReactions(numOfResponses, reactionProgressions)
}

// newTestController creates a new test controller injected with fake clients
// and returns:
//
// - a fake kubernetes core api client
// - a fake service catalog api client
// - a fake osb client, with configuration for happy path testing
// - a test controller
// - the shared informers for the service catalog v1beta1 api
// - a function for shutting down the API server
// - a function for shutting down the controller.
//
// If there is an error, newTestController calls 'Fatal' on the injected
// testing.T.
func newTestController(t *testing.T) (
	*fake.Clientset,
	clientset.Interface,
	*restclient.Config,
	*fakeosb.FakeClient,
	controller.Controller,
	informers.Interface,
	func(),
	func()) {

	// create a fake kube client
	fakeKubeClient := &fake.Clientset{}
	prependGetSecretNotFoundReaction(fakeKubeClient)

	// create an sc client and running server
	catalogClient, catalogClientConfig, shutdownServer := getFreshApiserverAndClient(t, server.StorageTypeEtcd.String(), func() runtime.Object {
		return &servicecatalog.ClusterServiceBroker{}
	})

	fakeOSBClient := fakeosb.NewFakeClient(getTestHappyPathBrokerClientConfig())
	brokerClFunc := fakeosb.ReturnFakeClientFunc(fakeOSBClient)

	// create informers
	informerFactory := scinformers.NewSharedInformerFactory(catalogClient, 10*time.Second)
	serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1beta1()

	// WARNING: Should you try to record more events than the buffer size
	// passed here, the recording function will hang indefinitely.
	fakeRecorder := record.NewFakeRecorder(50)

	// create a test controller
	testController, err := controller.NewController(
		fakeKubeClient,
		catalogClient.ServicecatalogV1beta1(),
		serviceCatalogSharedInformers.ClusterServiceBrokers(),
		serviceCatalogSharedInformers.ClusterServiceClasses(),
		serviceCatalogSharedInformers.ServiceInstances(),
		serviceCatalogSharedInformers.ServiceBindings(),
		serviceCatalogSharedInformers.ClusterServicePlans(),
		brokerClFunc,
		24*time.Hour,
		osb.LatestAPIVersion().HeaderValue(),
		fakeRecorder,
		7*24*time.Hour,
		7*24*time.Hour,
	)
	t.Log("controller start")
	if err != nil {
		t.Fatal(err)
	}

	stopCh := make(chan struct{})
	controllerStopped := make(chan struct{})
	go func() {
		testController.Run(1, stopCh)
		controllerStopped <- struct{}{}
	}()
	informerFactory.Start(stopCh)
	t.Log("informers start")

	shutdownController := func() {
		close(stopCh)
		<-controllerStopped
	}

	return fakeKubeClient, catalogClient, catalogClientConfig, fakeOSBClient, testController, serviceCatalogSharedInformers, shutdownServer, shutdownController
}

// changeUsernameForCatalogClient changes the name of the user that is using the catalog client
func changeUsernameForCatalogClient(catalogClient clientset.Interface, catalogClientConfig *restclient.Config, username string) (clientset.Interface, error) {
	catalogClientConfig.Username = username
	var err error
	catalogClient, err = clientset.NewForConfig(catalogClientConfig)
	if nil != err {
		return nil, fmt.Errorf("can't make the client from the config: %v", err)
	}
	return catalogClient, nil
}

// prependGetSecretNotFoundReaction prepends a reaction to getting secrets from the fake kube client
// that returns a not found error.
func prependGetSecretNotFoundReaction(fakeKubeClient *fake.Clientset) {
	fakeKubeClient.PrependReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), action.(clientgotesting.GetAction).GetName())
	})
}

// prependGetSecretReaction prepends a reaction to getting secrets from the fake kube client
// that returns a secret with the specified secret data when a request is made for the secret
// with the specified secret name.
func prependGetSecretReaction(fakeKubeClient *fake.Clientset, secretName string, secretData map[string][]byte) {
	fakeKubeClient.PrependReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		getAction, ok := action.(clientgotesting.GetAction)
		if !ok {
			return true, nil, apierrors.NewInternalError(fmt.Errorf("could not convert get secrets action to a GetAction: %T", action))
		}
		if getAction.GetName() != secretName {
			return false, nil, nil
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      secretName,
			},
			Data: secretData,
		}
		return true, secret, nil
	})
}

// getTestCatalogResponse returns a sample response to a get catalog request.
func getTestCatalogResponse() *osb.CatalogResponse {
	return &osb.CatalogResponse{
		Services: []osb.Service{
			{
				Name:        testClusterServiceClassName,
				ID:          "12345",
				Description: "a test service",
				Bindable:    true,
				Plans: []osb.Plan{
					{
						Name:        testClusterServicePlanName,
						Free:        truePtr(),
						ID:          testPlanExternalID,
						Description: "a test plan",
					},
					{
						Name:        testNonbindableClusterServicePlanName,
						Free:        truePtr(),
						ID:          testNonbindablePlanExternalID,
						Description: "an non-bindable test plan",
						Bindable:    falsePtr(),
					},
				},
			},
		},
	}
}

// getTestBindCredentials returns binding credentials to include in the response
// to a bind request.
func getTestBindCredentials() map[string]interface{} {
	return map[string]interface{}{
		"foo": "bar",
		"baz": "zap",
	}
}

// getTestHappyPathBrokerClientConfig returns configuration for the fake
// broker client that is appropriate for testing the synchronous happy path.
func getTestHappyPathBrokerClientConfig() fakeosb.FakeClientConfiguration {
	return fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: getTestCatalogResponse(),
		},
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{},
		},
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: getTestBindCredentials(),
			},
		},
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
		PollBindingLastOperationReaction: &fakeosb.PollBindingLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
		GetBindingReaction: &fakeosb.GetBindingReaction{
			Response: &osb.GetBindingResponse{
				Credentials: getTestBindCredentials(),
			},
		},
	}
}

// getTestBroker returns a ClusterServiceBroker to use for testing.
func getTestBroker() *v1beta1.ClusterServiceBroker {
	return &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testClusterServiceBrokerName},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			URL: testBrokerURL,
		},
	}
}

// verifyBrokerCreated verifies that the specified broker has been created
// and reconciled successfully.
func verifyBrokerCreated(t *testing.T, client clientsetsc.ServicecatalogV1beta1Interface, broker *v1beta1.ClusterServiceBroker) *v1beta1.ClusterServiceBroker {
	if err := util.WaitForBrokerCondition(client,
		broker.Name,
		v1beta1.ServiceBrokerCondition{
			Type:   v1beta1.ServiceBrokerConditionReady,
			Status: v1beta1.ConditionTrue,
		}); err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	if err := util.WaitForClusterServiceClassToExist(client, testClusterServiceClassGUID); err != nil {
		t.Fatalf("error waiting from ClusterServiceClass to exist: %v", err)
	}

	retBroker, err := client.ClusterServiceBrokers().Get(broker.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting the broker from storage: %v", err)
	}

	return retBroker
}

// deleteBroker deletes the specified broker.
func deleteBroker(t *testing.T, client clientsetsc.ServicecatalogV1beta1Interface, broker *v1beta1.ClusterServiceBroker) {
	if err := client.ClusterServiceBrokers().Delete(broker.Name, &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("broker should be deleted (%s)", err)
	}

	if err := util.WaitForClusterServiceClassToNotExist(client, testClusterServiceClassName); err != nil {
		t.Fatalf("error waiting for ClusterServiceClass to not exist: %v", err)
	}

	if err := util.WaitForBrokerToNotExist(client, broker.Name); err != nil {
		t.Fatalf("error waiting for Broker to not exist: %v", err)
	}
}

// getTestInstance returns a ServiceInstance to use for testing.
func getTestInstance() *v1beta1.ServiceInstance {
	return &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID: testExternalID,
		},
	}
}

// verifyInstanceCreated verifies that the specified instance has been created
// and reconciled successfully.
func verifyInstanceCreated(t *testing.T, client clientsetsc.ServicecatalogV1beta1Interface, instance *v1beta1.ServiceInstance) *v1beta1.ServiceInstance {
	if err := util.WaitForInstanceCondition(client, instance.Namespace, instance.Name, v1beta1.ServiceInstanceCondition{
		Type:   v1beta1.ServiceInstanceConditionReady,
		Status: v1beta1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become ready: %v", err)
	}

	retInst, err := client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if e, a := instance.Spec.ExternalID, retInst.Spec.ExternalID; e != a {
		t.Fatalf("returned OSB GUID '%s' doesn't match original '%s'", e, a)
	}

	return retInst
}

// deleteInstance deletes the specified instance
func deleteInstance(t *testing.T, client clientsetsc.ServicecatalogV1beta1Interface, instance *v1beta1.ServiceInstance) {
	if err := client.ServiceInstances(instance.Namespace).Delete(instance.Name, &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("instance delete should have been accepted: %v", err)
	}

	if err := util.WaitForInstanceToNotExist(client, instance.Namespace, instance.Name); err != nil {
		t.Fatalf("error waiting for instance to be deleted: %v", err)
	}
}

// getTestBinding gets a ServiceBinding to use for testing.
func getTestBinding() *v1beta1.ServiceBinding {
	return &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testBindingName},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{
				Name: testInstanceName,
			},
		},
	}
}

// verifyBindingCreated verifies that the specified binding has been created
// and reconciled successfully.
func verifyBindingCreated(t *testing.T, client clientsetsc.ServicecatalogV1beta1Interface, binding *v1beta1.ServiceBinding) *v1beta1.ServiceBinding {
	condition := v1beta1.ServiceBindingCondition{
		Type:   v1beta1.ServiceBindingConditionReady,
		Status: v1beta1.ConditionTrue,
	}
	if cond, err := util.WaitForBindingCondition(client, testNamespace, testBindingName, condition); err != nil {
		t.Fatalf("error waiting for binding condition: %v\n"+"expecting: %+v\n"+"last seen: %+v", err, condition, cond)
	}

	retBinding, err := client.ServiceBindings(binding.Namespace).Get(binding.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting binding %s/%s back", binding.Namespace, binding.Name)
	}

	return retBinding
}

// deleteBinding deletes the specified binding.
func deleteBinding(t *testing.T, client clientsetsc.ServicecatalogV1beta1Interface, binding *v1beta1.ServiceBinding) {
	if err := client.ServiceBindings(binding.Namespace).Delete(binding.Name, &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("binding delete should have been accepted: %v", err)
	}

	if err := util.WaitForBindingToNotExist(client, binding.Namespace, binding.Name); err != nil {
		t.Fatalf("error waiting for binding to not exist: %v", err)
	}
}

// controllerTest is used to set-up, run, and tear-down an integration test
// that tests the functionality of the controller.
type controllerTest struct {
	// testing.T for the integration test being run
	t *testing.T

	// fake kube client
	kubeClient *fake.Clientset
	// fake catalog client
	catalogClient clientset.Interface
	// fake catalog client configuration
	catalogClientConfig *restclient.Config
	// fake service catalog client
	client clientsetsc.ServicecatalogV1beta1Interface
	// fake osb broker client
	osbClient *fakeosb.FakeClient
	// fake controller
	controller controller.Controller
	// fake informers
	informers informers.Interface

	// the broker to create.
	// After the broker is created and verified, this is the broker from storage.
	// This is not updated after creation, so will not reflect any updates.
	broker *v1beta1.ClusterServiceBroker
	// the instance to create.
	// After the instance is created and verified, this is the instance from storage
	// This is not updated after creation, so will not reflect any updates.
	instance *v1beta1.ServiceInstance
	// the binding to create
	// After the binding is created and verified, this is the binding from storage
	// This is not updated after creation, so will not reflect any updates.
	binding *v1beta1.ServiceBinding

	// true if the verification that the broker was created and reconciled
	// successfully should be skipped. This is useful for tests where the
	// reconciliation is expected to fail.
	skipVerifyingBrokerSuccess bool
	// true if the verification that the broker was created and reconciled
	// successfully should be skipped. This is useful for tests where the
	// reconciliation is expected to fail.
	skipVerifyingInstanceSuccess bool
	// true if the verification that the broker was created and reconciled
	// successfully should be skipped. This is useful for tests where the
	// reconciliation is expected to fail.
	skipVerifyingBindingSuccess bool

	// function to run before creating any resources. This is useful for setting
	// up reactions for the fake components.
	setup func(t *controllerTest)
	// function to run just prior to creating the broker. This will only be run
	// if there is actually a broker to create.
	preCreateBroker func(t *controllerTest)
	// function to run just after creating, and optionally verifying, the broker.
	// This will only be run if there is actually a broker to create.
	postCreateBroker func(t *controllerTest)
	// function to run just prior to deleting the broker. This will only be run
	// if there was a broker created.
	preDeleteBroker func(t *controllerTest)
	// function to run just after deleting the broker. This will only be run
	// if there was a broker created.
	postDeleteBroker func(t *controllerTest)
	// function to run before creating the instance. This will only be run
	// if there is actually an instance to create.
	preCreateInstance func(t *controllerTest)
	// function to run just after creating, and optionally verifying, the instance.
	// This will only be run if there is actually an instance to create.
	postCreateInstance func(t *controllerTest)
	// function to run just prior to deleting the instance. This will only be run
	// if there was an instance created.
	preDeleteInstance func(t *controllerTest)
	// function to run just after deleting the instance. This will only be run
	// if there was an instance created.
	postDeleteInstance func(t *controllerTest)
	// function to run before creating the binding. This will only be run
	// if there is actually a binding to create.
	preCreateBinding func(t *controllerTest)
	// function to run just after creating, and optionally verifying, the binding.
	// This will only be run if there is actually a binding to create.
	postCreateBinding func(t *controllerTest)
	// function to run just prior to deleting the binding. This will only be run
	// if there was a binding created.
	preDeleteBinding func(t *controllerTest)
	// function to run just after deleting the binding. This will only be run
	// if there was a binding created.
	postDeleteBinding func(t *controllerTest)
}

// run executes the test.
//
// Steps performed:
// - create controller and API server with fakes
// - call setup function
// - create broker
// - create instance
// - create binding
// - call supplied test function
// - delete binding
// - delete instance
// - delete broker
// - clean up controller and API server
func (ct *controllerTest) run(test func(*controllerTest)) {
	kubeClient, catalogClient, catalogClientConfig, osbClient, controller, informers, shutdownServer, shutdownController := newTestController(ct.t)
	defer shutdownController()
	defer shutdownServer()

	ct.kubeClient = kubeClient
	ct.catalogClient = catalogClient
	ct.catalogClientConfig = catalogClientConfig
	ct.osbClient = osbClient
	ct.controller = controller
	ct.informers = informers

	ct.client = catalogClient.ServicecatalogV1beta1()

	if ct.setup != nil {
		ct.setup(ct)
	}

	if ct.broker != nil {
		if ct.preCreateBroker != nil {
			ct.preCreateBroker(ct)
		}
		_, err := ct.client.ClusterServiceBrokers().Create(ct.broker)
		if nil != err {
			ct.t.Fatalf("error creating the broker %q (%q)", ct.broker.Name, err)
		}
		if !ct.skipVerifyingBrokerSuccess {
			ct.broker = verifyBrokerCreated(ct.t, ct.client, ct.broker)
		}
		if ct.postCreateBroker != nil {
			ct.postCreateBroker(ct)
		}
	}

	if ct.instance != nil {
		if ct.preCreateInstance != nil {
			ct.preCreateInstance(ct)
		}
		if _, err := ct.client.ServiceInstances(ct.instance.Namespace).Create(ct.instance); err != nil {
			ct.t.Fatalf("error creating Instance: %v", err)
		}
		if !ct.skipVerifyingInstanceSuccess {
			ct.instance = verifyInstanceCreated(ct.t, ct.client, ct.instance)
		}
		if ct.postCreateInstance != nil {
			ct.postCreateInstance(ct)
		}
	}

	if ct.binding != nil {
		if ct.preCreateBinding != nil {
			ct.preCreateBinding(ct)
		}
		_, err := ct.client.ServiceBindings(ct.binding.Namespace).Create(ct.binding)
		if err != nil {
			ct.t.Fatalf("error creating Binding: %v", err)
		}
		if !ct.skipVerifyingBindingSuccess {
			ct.binding = verifyBindingCreated(ct.t, ct.client, ct.binding)
		}
		if ct.postCreateBinding != nil {
			ct.postCreateBinding(ct)
		}
	}

	if test != nil {
		test(ct)
	}

	if ct.binding != nil {
		if ct.preDeleteBinding != nil {
			ct.preDeleteBinding(ct)
		}
		deleteBinding(ct.t, ct.client, ct.binding)
		if ct.postDeleteBinding != nil {
			ct.postDeleteBinding(ct)
		}
	}

	if ct.instance != nil {
		if ct.preDeleteInstance != nil {
			ct.preDeleteInstance(ct)
		}
		deleteInstance(ct.t, ct.client, ct.instance)
		if ct.postDeleteInstance != nil {
			ct.postDeleteInstance(ct)
		}
	}

	if ct.broker != nil {
		if ct.preDeleteBroker != nil {
			ct.preDeleteBroker(ct)
		}
		deleteBroker(ct.t, ct.client, ct.broker)
		if ct.postDeleteBroker != nil {
			ct.postDeleteBroker(ct)
		}
	}
}

// getLastBrokerActions gets the last action made to the fake broker client.
// It also verifies that the last action had the specified action type.
func getLastBrokerAction(t *testing.T, osbClient *fakeosb.FakeClient, actionType fakeosb.ActionType) fakeosb.Action {
	brokerActions := osbClient.Actions()
	if len(brokerActions) == 0 {
		t.Fatalf("no broker actions")
	}
	brokerAction := brokerActions[len(brokerActions)-1]
	if e, a := actionType, brokerAction.Type; e != a {
		t.Fatalf("unexpected action type: expected %s, got %s", e, a)
	}
	return brokerAction
}

// convertParametersIntoRawExtension converts the specified map of parameters
// into a RawExtension object that can be used in the Parameters field of
// ServiceInstanceSpec or ServiceBindingSpec.
func convertParametersIntoRawExtension(t *testing.T, parameters map[string]interface{}) *runtime.RawExtension {
	marshalledParams, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	return &runtime.RawExtension{Raw: marshalledParams}
}
