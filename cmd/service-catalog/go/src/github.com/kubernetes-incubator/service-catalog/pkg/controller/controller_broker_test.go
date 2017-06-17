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

package controller

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	fakebrokerapi "github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/fake"
	fakebrokerserver "github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/fake/server"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	clientgotesting "k8s.io/client-go/testing"
)

func TestShouldReconcileBroker(t *testing.T) {
	cases := []struct {
		name      string
		broker    *v1alpha1.Broker
		now       time.Time
		interval  time.Duration
		reconcile bool
	}{
		{
			name:      "no status",
			broker:    getTestBroker(),
			now:       time.Now(),
			interval:  3 * time.Minute,
			reconcile: true,
		},
		{
			name: "no ready condition",
			broker: func() *v1alpha1.Broker {
				b := getTestBroker()
				b.Status = v1alpha1.BrokerStatus{
					Conditions: []v1alpha1.BrokerCondition{
						{
							Type:   v1alpha1.BrokerConditionType("NotARealCondition"),
							Status: v1alpha1.ConditionTrue,
						},
					},
				}
				return b
			}(),
			now:       time.Now(),
			interval:  3 * time.Minute,
			reconcile: true,
		},
		{
			name:      "not ready",
			broker:    getTestBrokerWithStatus(v1alpha1.ConditionFalse),
			now:       time.Now(),
			interval:  3 * time.Minute,
			reconcile: true,
		},
		{
			name: "ready, interval elapsed",
			broker: func() *v1alpha1.Broker {
				broker := getTestBrokerWithStatus(v1alpha1.ConditionTrue)
				return broker
			}(),
			now:       time.Now(),
			interval:  3 * time.Minute,
			reconcile: true,
		},
		{
			name: "ready, interval not elapsed",
			broker: func() *v1alpha1.Broker {
				broker := getTestBrokerWithStatus(v1alpha1.ConditionTrue)
				return broker
			}(),
			now:       time.Now(),
			interval:  3 * time.Hour,
			reconcile: false,
		},
	}

	for _, tc := range cases {
		var ltt *time.Time
		if len(tc.broker.Status.Conditions) != 0 {
			ltt = &tc.broker.Status.Conditions[0].LastTransitionTime.Time
		}

		t.Logf("%v: now: %v, interval: %v, last transition time: %v", tc.name, tc.now, tc.interval, ltt)
		actual := shouldReconcileBroker(tc.broker, tc.now, tc.interval)

		if e, a := tc.reconcile, actual; e != a {
			t.Errorf("%v: unexpected result: expected %v, got %v", tc.name, e, a)
		}
	}
}

func TestReconcileBroker(t *testing.T) {
	const (
		brokerUsername = "testuser"
		brokerPassword = "testpassword"
	)
	controllerParams, err := newTestControllerWithBrokerServer(brokerUsername, brokerPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer controllerParams.Close()

	controllerParams.BrokerServerHandler.Catalog = fakebrokerserver.ConvertCatalog(getTestCatalog())
	controllerParams.Controller.reconcileBroker(getTestBroker())

	actions := controllerParams.FakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	// first action should be a create action for a service class
	assertCreate(t, actions[0], getTestServiceClass())

	// second action should be an update action for broker status subresource
	updatedBroker := assertUpdateStatus(t, actions[1], getTestBroker())
	assertBrokerReadyTrue(t, updatedBroker)

	// verify no kube resources created
	assertNumberOfActions(t, controllerParams.FakeKubeClient.Actions(), 0)

	events := getRecordedEvents(controllerParams.Controller)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successFetchedCatalogReason + " " + successFetchedCatalogMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}

	if controllerParams.BrokerServerHandler.CatalogRequests != 1 {
		t.Fatalf(
			"expected 1 catalog request, got %d",
			controllerParams.BrokerServerHandler.CatalogRequests,
		)
	}
}

func TestReconcileBrokerExistingServiceClass(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	testServiceClass := getTestServiceClass()
	sharedInformers.ServiceClasses().Informer().GetStore().Add(testServiceClass)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	testController.reconcileBroker(getTestBroker())

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	// first action should be an update action for a service class
	assertUpdate(t, actions[0], testServiceClass)

	// second action should be an update action for broker status subresource
	updatedBroker := assertUpdateStatus(t, actions[1], getTestBroker())
	assertBrokerReadyTrue(t, updatedBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

func TestReconcileBrokerExistingServiceClassDifferentExternalID(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	testServiceClass := getTestServiceClass()
	testServiceClass.ExternalID = "notTheSame"
	sharedInformers.ServiceClasses().Informer().GetStore().Add(testServiceClass)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	testController.reconcileBroker(getTestBroker())

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedBroker := assertUpdateStatus(t, actions[0], getTestBroker())
	assertBrokerReadyFalse(t, updatedBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorSyncingCatalogReason + ` Error reconciling serviceClass "test-serviceclass" (broker "test-broker"): ServiceClass "test-serviceclass" already exists with OSB guid "notTheSame", received different guid "SCGUID"`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event; expected\n%v, got\n%v", e, a)
	}
}

func TestReconcileBrokerExistingServiceClassDifferentBroker(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	testServiceClass := getTestServiceClass()
	testServiceClass.BrokerName = "notTheSame"
	sharedInformers.ServiceClasses().Informer().GetStore().Add(testServiceClass)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	testController.reconcileBroker(getTestBroker())

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedBroker := assertUpdateStatus(t, actions[0], getTestBroker())
	assertBrokerReadyFalse(t, updatedBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorSyncingCatalogReason + ` Error reconciling serviceClass "test-serviceclass" (broker "test-broker"): ServiceClass "test-serviceclass" for Broker "test-broker" already exists for Broker "notTheSame"`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event; expected\n%v, got\n%v", e, a)
	}
}

func TestReconcileBrokerDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	testServiceClass := getTestServiceClass()
	sharedInformers.ServiceClasses().Informer().GetStore().Add(testServiceClass)

	broker := getTestBroker()
	broker.DeletionTimestamp = &metav1.Time{}
	broker.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}

	testController.reconcileBroker(broker)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Deleting the associated ServiceClass
	// 1. Updating the ready condition
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	assertDelete(t, actions[0], testServiceClass)

	updatedBroker := assertUpdateStatus(t, actions[1], broker)
	assertBrokerReadyFalse(t, updatedBroker)

	updatedBroker = assertUpdateStatus(t, actions[2], broker)
	assertEmptyFinalizers(t, updatedBroker)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successBrokerDeletedReason + " " + "The broker test-broker was deleted successfully."
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBrokerErrorFetchingCatalog(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, _ := newTestController(t)

	fakeBrokerClient.CatalogClient.RetErr = fakebrokerapi.ErrInstanceNotFound
	broker := getTestBroker()

	testController.reconcileBroker(broker)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedBroker := assertUpdateStatus(t, actions[0], broker)
	assertBrokerReadyFalse(t, updatedBroker)

	assertNumberOfActions(t, fakeKubeClient.Actions(), 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorFetchingCatalogReason + " " + "Error getting broker catalog for broker \"test-broker\": instance not found"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBrokerZeroServices(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, _ := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = &brokerapi.Catalog{
		Services: []*brokerapi.Service{},
	}
	broker := getTestBroker()

	testController.reconcileBroker(broker)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedBroker := assertUpdateStatus(t, actions[0], broker)
	assertBrokerReadyFalse(t, updatedBroker)

	assertNumberOfActions(t, fakeKubeClient.Actions(), 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorSyncingCatalogReason + ` Error getting catalog payload for broker "test-broker"; received zero services; at least one service is required`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event; \nexpected: %v\ngot:     %v", e, a)
	}
}

func TestReconcileBrokerWithAuthError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t)

	broker := getTestBroker()
	broker.Spec.AuthInfo = &v1alpha1.BrokerAuthInfo{
		BasicAuthSecret: &v1.ObjectReference{
			Namespace: "does_not_exist",
			Name:      "auth-name",
		},
	}

	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("no secret defined")
	})

	testController.reconcileBroker(broker)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedBroker := assertUpdateStatus(t, actions[0], broker)
	assertBrokerReadyFalse(t, updatedBroker)

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	getAction := kubeActions[0].(clientgotesting.GetAction)
	if e, a := "get", getAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; expected %v, got %v", e, a)
	}
	if e, a := "secrets", getAction.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; expected %v, got %v", e, a)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorAuthCredentialsReason + " " + "Error getting broker auth credentials for broker \"test-broker\": no secret defined"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBrokerWithReconcileError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t)

	broker := getTestBroker()
	broker.Spec.AuthInfo = &v1alpha1.BrokerAuthInfo{
		BasicAuthSecret: &v1.ObjectReference{
			Namespace: "does_not_exist",
			Name:      "auth-name",
		},
	}

	fakeCatalogClient.AddReactor("create", "serviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("error creating serviceclass")
	})

	testController.reconcileBroker(broker)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedBroker := assertUpdateStatus(t, actions[0], broker)
	assertBrokerReadyFalse(t, updatedBroker)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	getAction := kubeActions[0].(clientgotesting.GetAction)
	if e, a := "get", getAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; expected %v, got %v", e, a)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorAuthCredentialsReason + " " + "Error getting broker auth credentials for broker \"test-broker\": auth secret didn't contain username"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestUpdateBrokerCondition(t *testing.T) {
	cases := []struct {
		name                  string
		input                 *v1alpha1.Broker
		status                v1alpha1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestBroker(),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestBrokerWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready with reason and message change",
			input:                 getTestBrokerWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestBrokerWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionTrue,
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestBrokerWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionTrue,
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestBrokerWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, _, testController, _ := newTestController(t)

		clone, err := api.Scheme.DeepCopy(tc.input)
		if err != nil {
			t.Errorf("%v: deep copy failed", tc.name)
			continue
		}

		inputClone := clone.(*v1alpha1.Broker)

		err = testController.updateBrokerCondition(tc.input, v1alpha1.BrokerConditionReady, tc.status, tc.reason, tc.message)
		if err != nil {
			t.Errorf("%v: error updating broker condition: %v", tc.name, err)
			continue
		}

		if !reflect.DeepEqual(tc.input, inputClone) {
			t.Errorf("%v: updating broker condition mutated input: expected %v, got %v", tc.name, inputClone, tc.input)
			continue
		}

		actions := fakeCatalogClient.Actions()
		if ok := expectNumberOfActions(t, tc.name, actions, 1); !ok {
			continue
		}

		updatedBroker, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedBroker.(*v1alpha1.Broker)
		if !ok {
			t.Errorf("%v: couldn't convert to broker", tc.name)
			continue
		}

		var initialTs metav1.Time
		if len(inputClone.Status.Conditions) != 0 {
			initialTs = inputClone.Status.Conditions[0].LastTransitionTime
		}

		if e, a := 1, len(updateActionObject.Status.Conditions); e != a {
			t.Errorf("%v: expected %v condition(s), got %v", tc.name, e, a)
		}

		outputCondition := updateActionObject.Status.Conditions[0]
		newTs := outputCondition.LastTransitionTime

		if tc.transitionTimeChanged && initialTs == newTs {
			t.Errorf("%v: transition time didn't change when it should have", tc.name)
			continue
		} else if !tc.transitionTimeChanged && initialTs != newTs {
			t.Errorf("%v: transition time changed when it shouldn't have", tc.name)
			continue
		}
		if e, a := tc.reason, outputCondition.Reason; e != "" && e != a {
			t.Errorf("%v: condition reasons didn't match; expected %v, got %v", tc.name, e, a)
			continue
		}
		if e, a := tc.message, outputCondition.Message; e != "" && e != a {
			t.Errorf("%v: condition reasons didn't match; expected %v, got %v", tc.name, e, a)
		}
	}
}
