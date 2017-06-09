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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	fakebrokerapi "github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/fake"
	servicecataloginformers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions"
	v1alpha1informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1alpha1"

	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"

	clientgofake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
)

const (
	serviceClassGUID            = "SCGUID"
	planGUID                    = "PGUID"
	nonbindableServiceClassGUID = "UNBINDABLE-SERVICE"
	nonbindablePlanGUID         = "UNBINDABLE-PLAN"
	instanceGUID                = "IGUID"
	bindingGUID                 = "BGUID"

	testBrokerName                  = "test-broker"
	testServiceClassName            = "test-serviceclass"
	testNonbindableServiceClassName = "test-unbindable-serviceclass"
	testPlanName                    = "test-plan"
	testNonbindablePlanName         = "test-unbindable-plan"
	testInstanceName                = "test-instance"
	testBindingName                 = "test-binding"
	testNamespace                   = "test-ns"
	testBindingSecretName           = "test-secret"
	testOperation                   = "test-operation"
	testDashboardURL                = "http://dashboard"
)

const testCatalog = `{
  "services": [{
    "name": "fake-service",
    "id": "acb56d7c-XXXX-XXXX-XXXX-feb140a59a66",
    "description": "fake service",
    "tags": ["no-sql", "relational"],
    "requires": ["route_forwarding"],
    "max_db_per_node": 5,
    "bindable": true,
    "metadata": {
      "provider": {
        "name": "The name"
      },
      "listing": {
        "imageUrl": "http://example.com/cat.gif",
        "blurb": "Add a blurb here",
        "longDescription": "A long time ago, in a galaxy far far away..."
      },
      "displayName": "The Fake Broker"
    },
    "dashboard_client": {
      "id": "398e2f8e-XXXX-XXXX-XXXX-19a71ecbcf64",
      "secret": "277cabb0-XXXX-XXXX-XXXX-7822c0a90e5d",
      "redirect_uri": "http://localhost:1234"
    },
    "plan_updateable": true,
    "plans": [{
      "name": "fake-plan-1",
      "id": "d3031751-XXXX-XXXX-XXXX-a42377d3320e",
      "description": "Shared fake Server, 5tb persistent disk, 40 max concurrent connections",
      "max_storage_tb": 5,
      "metadata": {
        "costs":[
            {
               "amount":{
                  "usd":99.0
               },
               "unit":"MONTHLY"
            },
            {
               "amount":{
                  "usd":0.99
               },
               "unit":"1GB of messages over 20GB"
            }
         ],
        "bullets": [
            "Shared fake server",
            "5 TB storage",
            "40 concurrent connections"
        ]
      }
    }, {
      "name": "fake-plan-2",
      "id": "0f4008b5-XXXX-XXXX-XXXX-dace631cd648",
      "description": "Shared fake Server, 5tb persistent disk, 40 max concurrent connections. 100 async",
      "max_storage_tb": 5,
      "metadata": {
        "costs":[
            {
               "amount":{
                  "usd":199.0
               },
               "unit":"MONTHLY"
            },
            {
               "amount":{
                  "usd":0.99
               },
               "unit":"1GB of messages over 20GB"
            }
         ],
        "bullets": [
          "40 concurrent connections"
        ]
      }
    }]
  }]
}`

const testCatalogWithMultipleServices = `{
  "services": [
    {
      "name": "service1",
      "description": "service 1 description",
      "metadata": {
        "field1": "value1"
      },
      "plans": [{
        "name": "s1plan1",
        "id": "s1_plan1_id",
        "description": "s1 plan1 description"
      },
      {
        "name": "s1plan2",
        "id": "s1_plan2_id",
        "description": "s1 plan2 description",
        "metadata": {
          "planmeta": "planvalue"
        }
      }]
    },
    {
      "name": "service2",
      "description": "service 2 description",
      "metadata": ["first", "second", "third"],
      "plans": [{
        "name": "s2plan1",
        "id": "s2_plan1_id",
        "description": "s2 plan1 description"
      },
      {
        "name": "s2plan2",
        "id": "s2_plan2_id",
        "description": "s2 plan2 description",
        "metadata": {
          "planmeta": "planvalue"
      }
      }]
    }
]}`

// broker used in most of the tests that need a broker
func getTestBroker() *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1alpha1.BrokerSpec{
			URL: "https://example.com",
		},
	}
}

func getTestBrokerWithStatus(status v1alpha1.ConditionStatus) *v1alpha1.Broker {
	broker := getTestBroker()
	broker.Status = v1alpha1.BrokerStatus{
		Conditions: []v1alpha1.BrokerCondition{{
			Type:               v1alpha1.BrokerConditionReady,
			Status:             status,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
	}

	return broker
}

// a bindable service class wired to the result of getTestBroker()
func getTestServiceClass() *v1alpha1.ServiceClass {
	return &v1alpha1.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceClassName},
		BrokerName: testBrokerName,
		ExternalID: serviceClassGUID,
		Bindable:   true,
		Plans: []v1alpha1.ServicePlan{
			{
				Name:       testPlanName,
				Free:       true,
				ExternalID: planGUID,
			},
			{
				Name:       testNonbindablePlanName,
				Free:       true,
				ExternalID: nonbindablePlanGUID,
				Bindable:   falsePtr(),
			},
		},
	}
}

// an unbindable service class wired to the result of getTestBroker()
func getTestNonbindableServiceClass() *v1alpha1.ServiceClass {
	return &v1alpha1.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: testNonbindableServiceClassName},
		BrokerName: testBrokerName,
		ExternalID: nonbindableServiceClassGUID,
		Bindable:   false,
		Plans: []v1alpha1.ServicePlan{
			{
				Name:       testPlanName,
				Free:       true,
				ExternalID: planGUID,
				Bindable:   truePtr(),
			},
			{
				Name:       testNonbindablePlanName,
				Free:       true,
				ExternalID: nonbindablePlanGUID,
			},
		},
	}
}

// broker catalog that provides the service class named in of
// getTestServiceClass()
func getTestCatalog() *brokerapi.Catalog {
	return &brokerapi.Catalog{
		Services: []*brokerapi.Service{
			{
				Name:        testServiceClassName,
				ID:          serviceClassGUID,
				Description: "a test service",
				Bindable:    true,
				Plans: []brokerapi.ServicePlan{
					{
						Name:        testPlanName,
						Free:        true,
						ID:          planGUID,
						Description: "a test plan",
					},
				},
			},
		},
	}
}

// instance referencing the result of getTestServiceClass()
func getTestInstance() *v1alpha1.Instance {
	return &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName, Namespace: testNamespace},
		Spec: v1alpha1.InstanceSpec{
			ServiceClassName: testServiceClassName,
			PlanName:         testPlanName,
			ExternalID:       instanceGUID,
		},
	}
}

// an instance referencing the result of getTestNonbindableServiceClass, on the non-bindable plan.
func getTestNonbindableInstance() *v1alpha1.Instance {
	i := getTestInstance()
	i.Spec.ServiceClassName = testNonbindableServiceClassName
	i.Spec.PlanName = testNonbindablePlanName

	return i
}

// an instance referencing the result of getTestNonbindableServiceClass, on the bindable plan.
func getTestInstanceNonbindableServiceBindablePlan() *v1alpha1.Instance {
	i := getTestNonbindableInstance()
	i.Spec.PlanName = testPlanName

	return i
}

func getTestInstanceBindableServiceNonbindablePlan() *v1alpha1.Instance {
	i := getTestInstance()
	i.Spec.PlanName = testNonbindablePlanName

	return i
}

func getTestInstanceWithStatus(status v1alpha1.ConditionStatus) *v1alpha1.Instance {
	instance := getTestInstance()
	instance.Status = v1alpha1.InstanceStatus{
		Conditions: []v1alpha1.InstanceCondition{{
			Type:               v1alpha1.InstanceConditionReady,
			Status:             status,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
	}

	return instance
}

// getTestInstanceAsync returns an instance in async mode
func getTestInstanceAsyncProvisioning(operation string) *v1alpha1.Instance {
	instance := getTestInstance()
	if operation != "" {
		instance.Status.LastOperation = &operation
	}
	instance.Status = v1alpha1.InstanceStatus{
		Conditions: []v1alpha1.InstanceCondition{{
			Type:               v1alpha1.InstanceConditionReady,
			Status:             v1alpha1.ConditionFalse,
			Message:            "Provisioning",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
		AsyncOpInProgress: true,
	}

	return instance
}

func getTestInstanceAsyncDeprovisioning(operation string) *v1alpha1.Instance {
	instance := getTestInstance()
	if operation != "" {
		instance.Status.LastOperation = &operation
	}
	instance.Status = v1alpha1.InstanceStatus{
		Conditions: []v1alpha1.InstanceCondition{{
			Type:               v1alpha1.InstanceConditionReady,
			Status:             v1alpha1.ConditionFalse,
			Message:            "Deprovisioning",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
		AsyncOpInProgress: true,
	}

	// Set the deleted timestamp to simulate deletion
	ts := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	instance.DeletionTimestamp = &ts
	return instance
}

func getTestInstanceAsyncDeprovisioningWithFinalizer(operation string) *v1alpha1.Instance {
	instance := getTestInstanceAsyncDeprovisioning(operation)
	instance.ObjectMeta.Finalizers = []string{"kubernetes"}
	return instance
}

// binding referencing the result of getTestBinding()
func getTestBinding() *v1alpha1.Binding {
	return &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

}

type instanceParameters struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

type bindingParameters struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

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
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, _ := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	testController.reconcileBroker(getTestBroker())

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	// first action should be a create action for a service class
	assertCreate(t, actions[0], getTestServiceClass())

	// second action should be an update action for broker status subresource
	updatedBroker := assertUpdateStatus(t, actions[1], getTestBroker())
	assertBrokerReadyTrue(t, updatedBroker)

	// verify no kube resources created
	assertNumberOfActions(t, fakeKubeClient.Actions(), 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successFetchedCatalogReason + " " + successFetchedCatalogMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
	broker.Finalizers = []string{"kubernetes"}

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

func TestReconcileBrokerWithAuthError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t)

	broker := getTestBroker()
	broker.Spec.AuthSecret = &v1.ObjectReference{
		Namespace: "does_not_exist",
		Name:      "auth-name",
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
	broker.Spec.AuthSecret = &v1.ObjectReference{
		Namespace: "does_not_exist",
		Name:      "auth-name",
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

func TestReconcileInstanceNonExistentServiceClass(t *testing.T) {
	_, fakeCatalogClient, _, testController, _ := newTestController(t)

	instance := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
		Spec: v1alpha1.InstanceSpec{
			ServiceClassName: "nothere",
			PlanName:         "nothere",
			ExternalID:       instanceGUID,
		},
	}

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such class exists.
	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance, errorNonexistentServiceClassReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServiceClassReason + " " + "Instance \"/test-instance\" references a non-existent ServiceClass \"nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceNonExistentBroker(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such broker exists.
	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance, errorNonexistentBrokerReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentBrokerReason + " " + "Instance \"test-ns/test-instance\" references a non-existent broker \"test-broker\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceWithAuthError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	broker := getTestBroker()
	broker.Spec.AuthSecret = &v1.ObjectReference{
		Namespace: "does_not_exist",
		Name:      "auth-name",
	}
	sharedInformers.Brokers().Informer().GetStore().Add(broker)
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()

	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("no secret defined")
	})

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updateAction := actions[0].(clientgotesting.UpdateAction)
	if e, a := "update", updateAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; expected %v, got %v", e, a)
	}
	updateActionObject := updateAction.GetObject().(*v1alpha1.Instance)
	if e, a := testInstanceName, updateActionObject.Name; e != a {
		t.Fatalf("Unexpected name of instance created: expected %v, got %v", e, a)
	}
	if e, a := 1, len(updateActionObject.Status.Conditions); e != a {
		t.Fatalf("Unexpected number of conditions: expected %v, got %v", e, a)
	}
	if e, a := "ErrorGettingAuthCredentials", updateActionObject.Status.Conditions[0].Reason; e != a {
		t.Fatalf("Unexpected condition reason: expected %v, got %v", e, a)
	}

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

func TestReconcileInstanceNonExistentServicePlan(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
		Spec: v1alpha1.InstanceSpec{
			ServiceClassName: testServiceClassName,
			PlanName:         "nothere",
			ExternalID:       instanceGUID,
		},
	}

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such class exists.
	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance, errorNonexistentServicePlanReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServicePlanReason + " " + "Instance \"/test-instance\" references a non-existent ServicePlan \"nothere\" on ServiceClass \"test-serviceclass\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()

	parameters := instanceParameters{Name: "test-param", Args: make(map[string]string)}
	parameters.Args["first"] = "first-arg"
	parameters.Args["second"] = "second-arg"

	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyTrue(t, updatedInstance)

	updateObject, ok := updatedInstance.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.Instance")
	}

	// Verify parameters are what we'd expect them to be, basically name, map with two values in it.
	if len(updateObject.Spec.Parameters.Raw) == 0 {
		t.Fatalf("Parameters was unexpectedly empty")
	}
	if si, ok := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; !ok {
		t.Fatalf("Did not find the created Instance in fakeInstanceClient after creation")
	} else {
		if len(si.Parameters) == 0 {
			t.Fatalf("Expected parameters but got none")
		}
		if e, a := "test-param", si.Parameters["name"].(string); e != a {
			t.Fatalf("Unexpected name for parameters: expected %v, got %v", e, a)
		}
		argsMap := si.Parameters["args"].(map[string]interface{})
		if e, a := "first-arg", argsMap["first"].(string); e != a {
			t.Fatalf("Unexpected value in parameter map: expected %v, got %v", e, a)
		}
		if e, a := "second-arg", argsMap["second"].(string); e != a {
			t.Fatalf("Unexpected value in parameter map: expected %v, got %v", e, a)
		}
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceWithInvalidParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()
	parameters := instanceParameters{Name: "test-param", Args: make(map[string]string)}
	parameters.Args["first"] = "first-arg"
	parameters.Args["second"] = "second-arg"

	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	// corrupt the byte slice to begin with a '!' instead of an opening JSON bracket '{'
	b[0] = 0x21
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance)

	if si, notOK := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; notOK {
		t.Fatalf("Unexpectedly found created Instance: %+v in fakeInstanceClient after creation", si)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorWithParameters + " " + "Failed to unmarshal Instance parameters"
	if e, a := expectedEvent, events[0]; !strings.Contains(a, e) { // event contains RawExtension, so just compare error message
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceWithProvisionFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()
	parameters := instanceParameters{Name: "test-param", Args: make(map[string]string)}
	parameters.Args["first"] = "first-arg"
	parameters.Args["second"] = "second-arg"

	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	fakeBrokerClient.InstanceClient.CreateErr = errors.New("fake creation failure")

	testController.reconcileInstance(instance)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance)

	if si, notOK := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; notOK {
		t.Fatalf("Unexpectedly found created Instance: %+v in fakeInstanceClient after creation", si)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorProvisionCalledReason + " " + "Error provisioning Instance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at Broker \"test-broker\": fake creation failure"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstance(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()
	fakeBrokerClient.InstanceClient.DashboardURL = testDashboardURL

	testNsUID := "test_uid_foo"

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(testNsUID),
			},
		}, nil
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()

	testController.reconcileInstance(instance)

	// Since synchronous operation, must not make it into the polling queue.
	if testController.pollingQueue.Len() != 0 {
		t.Fatalf("Expected the polling queue to be empty")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyTrue(t, updatedInstance)

	if si, ok := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; !ok {
		t.Fatalf("Did not find the created Instance in fakeInstanceClient after creation")
	} else {
		if len(si.Parameters) > 0 {
			t.Fatalf("Unexpected parameters, expected none, got %+v", si.Parameters)
		}

		if testNsUID != si.OrganizationGUID {
			t.Fatalf("Unexpected OrganizationGUID: expected %q, got %q", testNsUID, si.OrganizationGUID)
		}
		if testNsUID != si.SpaceGUID {
			t.Fatalf("Unexpected SpaceGUID: expected %q, got %q", testNsUID, si.SpaceGUID)
		}

		assertInstanceDashboardURL(t, instance, testDashboardURL)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successProvisionReason + " " + successProvisionMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceAsynchronous(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()
	fakeBrokerClient.InstanceClient.DashboardURL = testDashboardURL

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("test_uid_foo"),
			},
		}, nil
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusAccepted
	// And specify that we want broker to return an operation
	fakeBrokerClient.InstanceClient.Operation = testOperation
	instance := getTestInstance()

	if testController.pollingQueue.Len() != 0 {
		t.Fatalf("Expected the polling queue to be empty")
	}

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance)

	if si, ok := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; !ok {
		t.Fatalf("Did not find the created Instance in fakeInstanceClient after creation")
	} else {
		if len(si.Parameters) > 0 {
			t.Fatalf("Unexpected parameters, expected none, got %+v", si.Parameters)
		}

		ns, _ := fakeKubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if string(ns.UID) != si.OrganizationGUID {
			t.Fatalf("Unexpected OrganizationGUID: expected %q, got %q", string(ns.UID), si.OrganizationGUID)
		}
		if string(ns.UID) != si.SpaceGUID {
			t.Fatalf("Unexpected SpaceGUID: expected %q, got %q", string(ns.UID), si.SpaceGUID)
		}
	}

	// The item should've been added to the pollingQueue for later processing
	if testController.pollingQueue.Len() != 1 {
		t.Fatalf("Expected the asynchronous instance to end up in the polling queue")
	}
	item, _ := testController.pollingQueue.Get()
	if item == nil {
		t.Fatalf("Did not get back a key from polling queue")
	}
	key := item.(string)
	expectedKey := fmt.Sprintf("%s/%s", instance.Namespace, instance.Name)
	if key != expectedKey {
		t.Fatalf("got key as %q expected %q", key, expectedKey)
	}
	assertAsyncOpInProgressTrue(t, updatedInstance)
	assertInstanceLastOperation(t, updatedInstance, testOperation)
	assertInstanceDashboardURL(t, updatedInstance, testDashboardURL)
}

func TestReconcileInstanceAsynchronousNoOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("test_uid_foo"),
			},
		}, nil
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusAccepted
	instance := getTestInstance()

	if testController.pollingQueue.Len() != 0 {
		t.Fatalf("Expected the polling queue to be empty")
	}

	testController.reconcileInstance(instance)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance)

	if si, ok := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; !ok {
		t.Fatalf("Did not find the created Instance in fakeInstanceClient after creation")
	} else {
		if len(si.Parameters) > 0 {
			t.Fatalf("Unexpected parameters, expected none, got %+v", si.Parameters)
		}

		ns, _ := fakeKubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if string(ns.UID) != si.OrganizationGUID {
			t.Fatalf("Unexpected OrganizationGUID: expected %q, got %q", string(ns.UID), si.OrganizationGUID)
		}
		if string(ns.UID) != si.SpaceGUID {
			t.Fatalf("Unexpected SpaceGUID: expected %q, got %q", string(ns.UID), si.SpaceGUID)
		}
	}

	// The item should've been added to the pollingQueue for later processing
	if testController.pollingQueue.Len() != 1 {
		t.Fatalf("Expected the asynchronous instance to end up in the polling queue")
	}
	item, _ := testController.pollingQueue.Get()
	if item == nil {
		t.Fatalf("Did not get back a key from polling queue")
	}
	key := item.(string)
	expectedKey := fmt.Sprintf("%s/%s", instance.Namespace, instance.Name)
	if key != expectedKey {
		t.Fatalf("got key as %q expected %q", key, expectedKey)
	}
	assertAsyncOpInProgressTrue(t, updatedInstance)
	assertInstanceLastOperation(t, updatedInstance, "")
}

func TestReconcileInstanceNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()

	testController.reconcileInstance(instance)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	assertUpdateStatus(t, actions[0], instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorFindingNamespaceInstanceReason + " " + "Failed to get namespace \"test-ns\" during instance create: No namespace"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileInstanceDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.InstanceClient.Instances = map[string]*brokerapi.ServiceInstance{
		instanceGUID: {},
	}

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestInstance()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{"kubernetes"}

	fakeCatalogClient.AddReactor("get", "instances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	testController.reconcileInstance(instance)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the instance
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyFalse(t, updatedInstance)

	assertGet(t, actions[1], instance)
	updatedInstance = assertUpdateStatus(t, actions[2], instance)
	assertEmptyFinalizers(t, updatedInstance)

	if _, ok := fakeBrokerClient.InstanceClient.Instances[instanceGUID]; ok {
		t.Fatalf("Found the deleted Instance in fakeInstanceClient after deletion")
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestPollServiceInstanceInProgressProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "in progress"}

	instance := getTestInstanceAsyncProvisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err == nil {
		t.Fatalf("Expected pollInstanceInternal to fail while in progress")
	}
	// Make sure we get an error which means it will get requeued.
	if !strings.Contains(err.Error(), "still in progress") {
		t.Fatalf("pollInstanceInternal failed but not with expected error, expected %q got %q", "still in progress", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

func TestPollServiceInstanceSuccessProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "succeeded"}

	instance := getTestInstanceAsyncProvisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollInstanceInternal failed: %s", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	// Instance should be ready and there no longer is an async operation
	// in place.
	assertInstanceReadyTrue(t, updatedInstance)
	assertAsyncOpInProgressFalse(t, updatedInstance)
}

func TestPollServiceInstanceFailureProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "failed"}

	instance := getTestInstanceAsyncProvisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollInstanceInternal failed: %s", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	// Instance should be not ready and there no longer is an async operation
	// in place.
	assertInstanceReadyFalse(t, updatedInstance)
	assertAsyncOpInProgressFalse(t, updatedInstance)
}

func TestPollServiceInstanceInProgressDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "in progress"}

	instance := getTestInstanceAsyncDeprovisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err == nil {
		t.Fatalf("Expected pollInstanceInternal to fail while in progress")
	}
	// Make sure we get an error which means it will get requeued.
	if !strings.Contains(err.Error(), "still in progress") {
		t.Fatalf("pollInstanceInternal failed but not with expected error, expected %q got %q", "still in progress", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

func TestPollServiceInstanceSuccessDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "succeeded"}

	instance := getTestInstanceAsyncDeprovisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollInstanceInternal failed: %s", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	// Instance should have been deprovisioned
	assertInstanceReadyCondition(t, updatedInstance, v1alpha1.ConditionFalse, successDeprovisionReason)
	assertAsyncOpInProgressFalse(t, updatedInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

func TestPollServiceInstanceFailureDeprovisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "failed"}

	instance := getTestInstanceAsyncDeprovisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollInstanceInternal failed: %s", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	// Instance should be set to unknown since the operation on the broker
	// failed.
	assertInstanceReadyCondition(t, updatedInstance, v1alpha1.ConditionUnknown, errorDeprovisionCalledReason)
	assertAsyncOpInProgressFalse(t, updatedInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

func TestPollServiceInstanceStatusGoneDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusGone
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "succeeded"}

	instance := getTestInstanceAsyncDeprovisioning(testOperation)

	err := testController.pollInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollInstanceInternal failed: %s", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	// Instance should have been deprovisioned
	assertInstanceReadyCondition(t, updatedInstance, v1alpha1.ConditionFalse, successDeprovisionReason)
	assertAsyncOpInProgressFalse(t, updatedInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

func TestPollServiceInstanceSuccessDeprovisioningWithOperationWithFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	// Specify we want asynchronous provisioning...
	fakeBrokerClient.InstanceClient.ResponseCode = http.StatusOK
	fakeBrokerClient.InstanceClient.LastOperationResponse = &brokerapi.LastOperationResponse{State: "succeeded"}

	instance := getTestInstanceAsyncDeprovisioningWithFinalizer(testOperation)
	// updateInstanceFinalizers fetches the latest object.
	fakeCatalogClient.AddReactor("get", "instances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.pollInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollInstanceInternal failed: %s", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the instance (updateFinalizers calls)
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedInstance := assertUpdateStatus(t, actions[0], instance)
	assertInstanceReadyCondition(t, updatedInstance, v1alpha1.ConditionFalse, successDeprovisionReason)

	// Instance should have been deprovisioned
	assertGet(t, actions[1], instance)
	updatedInstance = assertUpdateStatus(t, actions[2], instance)
	assertEmptyFinalizers(t, updatedInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

func TestUpdateInstanceCondition(t *testing.T) {
	getTestInstanceWithStatus := func(status v1alpha1.ConditionStatus) *v1alpha1.Instance {
		instance := getTestInstance()
		instance.Status = v1alpha1.InstanceStatus{
			Conditions: []v1alpha1.InstanceCondition{{
				Type:               v1alpha1.InstanceConditionReady,
				Status:             status,
				Message:            "message",
				LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			}},
		}

		return instance
	}

	cases := []struct {
		name                  string
		input                 *v1alpha1.Instance
		status                v1alpha1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestInstance(),
			status:                v1alpha1.ConditionFalse,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestInstanceWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready, reason and message change",
			input:                 getTestInstanceWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestInstanceWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionTrue,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestInstanceWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionTrue,
			message:               "message",
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestInstanceWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionFalse,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "message -> message2",
			input:                 getTestInstanceWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionFalse,
			message:               "message2",
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
		inputClone := clone.(*v1alpha1.Instance)

		err = testController.updateInstanceCondition(tc.input, v1alpha1.InstanceConditionReady, tc.status, tc.reason, tc.message)
		if err != nil {
			t.Errorf("%v: error updating instance condition: %v", tc.name, err)
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

		updatedInstance, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedInstance.(*v1alpha1.Instance)
		if !ok {
			t.Errorf("%v: couldn't convert to instance", tc.name)
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

func TestReconcileBindingNonExistingInstance(t *testing.T) {
	_, fakeCatalogClient, _, testController, _ := newTestController(t)

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: "nothere"},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such instance exists.
	updateAction := actions[0].(clientgotesting.UpdateAction)
	if e, a := "update", updateAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on actions[0]; expected %v, got %v", e, a)
	}
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding, errorNonexistentInstanceReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentInstanceReason + " " + "Binding \"/test-binding\" references a non-existent Instance \"/nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingNonExistingServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	instance := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName, Namespace: testNamespace},
		Spec: v1alpha1.InstanceSpec{
			ServiceClassName: "nothere",
			PlanName:         testPlanName,
			ExternalID:       instanceGUID,
		},
	}
	sharedInformers.Instances().Informer().GetStore().Add(instance)

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such service class.
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding, errorNonexistentServiceClassMessage)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServiceClassMessage + " " + "Binding \"test-ns/test-binding\" references a non-existent ServiceClass \"nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	testNsUID := "test_ns_uid"

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(testNsUID),
			},
		}, nil
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	parameters := bindingParameters{Name: "test-param"}
	parameters.Args = append(parameters.Args, "first-arg")
	parameters.Args = append(parameters.Args, "second-arg")
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	binding.Spec.Parameters = &runtime.RawExtension{Raw: b}

	testController.reconcileBinding(binding)

	if testNsUID != fakeBrokerClient.Bindings[fakebrokerapi.BindingsMapKey(instanceGUID, bindingGUID)].AppID {
		t.Fatalf("Unexpected broker AppID: expected %q, got %q", testNsUID, fakeBrokerClient.Bindings[instanceGUID+":"+bindingGUID].AppID)
	}

	bindResource := fakeBrokerClient.BindingRequests[fakebrokerapi.BindingsMapKey(instanceGUID, bindingGUID)].BindResource
	if appGUID := bindResource["app_guid"]; testNsUID != fmt.Sprintf("%v", appGUID) {
		t.Fatalf("Unexpected broker AppID: expected %q, got %q", testNsUID, appGUID)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyTrue(t, updatedBinding)

	updateObject, ok := updatedBinding.(*v1alpha1.Binding)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.Binding")
	}

	// Verify parameters are what we'd expect them to be, basically name, array with two values in it.
	if len(updateObject.Spec.Parameters.Raw) == 0 {
		t.Fatalf("Parameters was unexpectedly empty")
	}
	if b, ok := fakeBrokerClient.BindingClient.Bindings[fakebrokerapi.BindingsMapKey(instanceGUID, bindingGUID)]; !ok {
		t.Fatalf("Did not find the created Binding in fakeInstanceBinding after creation")
	} else {
		if len(b.Parameters) == 0 {
			t.Fatalf("Expected parameters, but got none")
		}
		if e, a := "test-param", b.Parameters["name"].(string); e != a {
			t.Fatalf("Unexpected name for parameters: expected %v, got %v", e, a)
		}
		argsArray := b.Parameters["args"].([]interface{})
		if len(argsArray) != 2 {
			t.Fatalf("Expected 2 elements in args array, but got %d", len(argsArray))
		}
		foundFirst := false
		foundSecond := false
		for _, el := range argsArray {
			if el.(string) == "first-arg" {
				foundFirst = true
			}
			if el.(string) == "second-arg" {
				foundSecond = true
			}
		}
		if !foundFirst {
			t.Fatalf("Failed to find 'first-arg' in array, was %v", argsArray)
		}
		if !foundSecond {
			t.Fatalf("Failed to find 'second-arg' in array, was %v", argsArray)
		}
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successInjectedBindResultReason + " " + successInjectedBindResultMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingNonbindableServiceClass(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestNonbindableServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestNonbindableInstance())

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding, errorNonbindableServiceClassReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonbindableServiceClassReason + ` Binding "test-ns/test-binding" references a non-bindable ServiceClass ("test-unbindable-serviceclass") and Plan ("test-unbindable-plan") combination`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingNonbindableServiceClassBindablePlan(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestNonbindableServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(func() *v1alpha1.Instance {
		i := getTestInstanceNonbindableServiceBindablePlan()
		i.Status = v1alpha1.InstanceStatus{
			Conditions: []v1alpha1.InstanceCondition{
				{
					Type:   v1alpha1.InstanceConditionReady,
					Status: v1alpha1.ConditionTrue,
				},
			},
		}
		return i
	}())

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyTrue(t, updatedBinding)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

func TestReconcileBindingBindableServiceClassNonbindablePlan(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t)

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestInstanceBindableServiceNonbindablePlan())

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding, errorNonbindableServiceClassReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonbindableServiceClassReason + ` Binding "test-ns/test-binding" references a non-bindable ServiceClass ("test-serviceclass") and Plan ("test-unbindable-plan") combination`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingFailsWithInstanceAsyncOngoing(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestInstanceAsyncProvisioning(""))

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	err := testController.reconcileBinding(binding)
	if err == nil {
		t.Fatalf("reconcileBinding did not fail with async operation ongoing")
	}

	if !strings.Contains(err.Error(), "Ongoing Asynchronous") {
		t.Fatalf("Did not get the expected error %q : got %q", "Ongoing Asynchronous", err)
	}

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	if !strings.Contains(events[0], "has ongoing asynchronous operation") {
		t.Fatalf("Did not find expected error %q : got %q", "has ongoing asynchronous operation", events[0])
	}
	if !strings.Contains(events[0], testNamespace+"/"+testInstanceName) {
		t.Fatalf("Did not find expected instance name : got %q", events[0])
	}
	if !strings.Contains(events[0], testNamespace+"/"+testBindingName) {
		t.Fatalf("Did not find expected binding name : got %q", events[0])
	}
}

func TestReconcileBindingInstanceNotReady(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("test_ns_uid"),
			},
		}, nil
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestInstance())

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	if _, ok := fakeBrokerClient.Bindings[fakebrokerapi.BindingsMapKey(instanceGUID, bindingGUID)]; ok {
		t.Fatalf("Unexpected broker binding call")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorInstanceNotReadyReason + " " + `Binding cannot begin because referenced instance "test-ns/test-instance" is not ready`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	fakeBrokerClient.CatalogClient.RetCatalog = getTestCatalog()

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestInstance())

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}

	testController.reconcileBinding(binding)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorFindingNamespaceInstanceReason + " " + "Failed to get namespace \"test-ns\" during binding: No namespace"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestReconcileBindingDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t)

	bindingsMapKey := fakebrokerapi.BindingsMapKey(instanceGUID, bindingGUID)

	fakeBrokerClient.BindingClient.Bindings = map[string]*brokerapi.ServiceBinding{bindingsMapKey: {}}

	sharedInformers.Brokers().Informer().GetStore().Add(getTestBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.Instances().Informer().GetStore().Add(getTestInstance())

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{"kubernetes"},
		},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
			SecretName:  testBindingSecretName,
		},
	}

	fakeCatalogClient.AddReactor("get", "bindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	testController.reconcileBinding(binding)

	kubeActions := fakeKubeClient.Actions()
	// The two actions should be:
	// 0. Getting the secret
	// 1. Deleting the secret
	assertNumberOfActions(t, kubeActions, 2)

	getAction := kubeActions[0].(clientgotesting.GetActionImpl)
	if e, a := "get", getAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[0]; expected %v, got %v", e, a)
	}

	if e, a := binding.Spec.SecretName, getAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: expected %v, got %v", e, a)
	}

	deleteAction := kubeActions[1].(clientgotesting.DeleteActionImpl)
	if e, a := "delete", deleteAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[1]; expected %v, got %v", e, a)
	}

	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: expected %v, got %v", e, a)
	}

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the binding in question
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedBinding := assertUpdateStatus(t, actions[0], binding)
	assertBindingReadyFalse(t, updatedBinding)

	assertGet(t, actions[1], binding)

	updatedBinding = assertUpdateStatus(t, actions[2], binding)
	assertEmptyFinalizers(t, updatedBinding)

	if _, ok := fakeBrokerClient.BindingClient.Bindings[bindingsMapKey]; ok {
		t.Fatalf("Found the deleted Binding in fakeBindingClient after deletion")
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successUnboundReason + " " + "This binding was deleted successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

func TestUpdateBindingCondition(t *testing.T) {
	getTestBindingWithStatus := func(status v1alpha1.ConditionStatus) *v1alpha1.Binding {
		instance := getTestBinding()
		instance.Status = v1alpha1.BindingStatus{
			Conditions: []v1alpha1.BindingCondition{{
				Type:               v1alpha1.BindingConditionReady,
				Status:             status,
				Message:            "message",
				LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			}},
		}

		return instance
	}

	cases := []struct {
		name                  string
		input                 *v1alpha1.Binding
		status                v1alpha1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestBinding(),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestBindingWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready, message and reason change",
			input:                 getTestBindingWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestBindingWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionTrue,
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestBindingWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionTrue,
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestBindingWithStatus(v1alpha1.ConditionTrue),
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
		inputClone := clone.(*v1alpha1.Binding)

		err = testController.updateBindingCondition(tc.input, v1alpha1.BindingConditionReady, tc.status, tc.reason, tc.message)
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

		updatedBinding, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedBinding.(*v1alpha1.Binding)
		if !ok {
			t.Errorf("%v: couldn't convert to binding", tc.name)
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

func TestEmptyCatalogConversion(t *testing.T) {
	serviceClasses, err := convertCatalog(&brokerapi.Catalog{})
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 0 {
		t.Fatalf("Expected 0 serviceclasses for empty catalog, but got: %d", len(serviceClasses))
	}
}

func TestCatalogConversion(t *testing.T) {
	catalog := &brokerapi.Catalog{}
	err := json.Unmarshal([]byte(testCatalog), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}
	serviceClasses, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 1 {
		t.Fatalf("Expected 1 serviceclasses for testCatalog, but got: %d", len(serviceClasses))
	}
	serviceClass := serviceClasses[0]
	if len(serviceClass.Plans) != 2 {
		t.Fatalf("Expected 2 plans for testCatalog, but got: %d", len(serviceClass.Plans))
	}

	checkPlan(serviceClass, 0, "fake-plan-1", "Shared fake Server, 5tb persistent disk, 40 max concurrent connections", t)
	checkPlan(serviceClass, 1, "fake-plan-2", "Shared fake Server, 5tb persistent disk, 40 max concurrent connections. 100 async", t)
}

func checkPlan(serviceClass *v1alpha1.ServiceClass, index int, planName, planDescription string, t *testing.T) {
	plan := serviceClass.Plans[index]
	if plan.Name != planName {
		t.Fatalf("Expected plan %d's name to be \"%s\", but was: %s", index, planName, plan.Name)
	}
	if plan.Description != planDescription {
		t.Fatalf("Expected plan %d's description to be \"%s\", but was: %s", index, planDescription, plan.Description)
	}
}

func TestCatalogConversionMultipleServiceClasses(t *testing.T) {
	catalog := &brokerapi.Catalog{}
	err := json.Unmarshal([]byte(testCatalogWithMultipleServices), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}

	serviceClasses, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 2 {
		t.Fatalf("Expected 2 serviceclasses for empty catalog, but got: %d", len(serviceClasses))
	}
	foundSvcMeta1 := false
	foundSvcMeta2 := false
	foundPlanMeta := false
	for _, sc := range serviceClasses {
		// For service1 make sure we have service level metadata with field1 = value1 as the blob
		// and for service1 plan s1plan2 we have planmeta = planvalue as the blob.
		if sc.Name == "service1" {
			if sc.Description != "service 1 description" {
				t.Fatalf("Expected service1's description to be \"service 1 description\", but was: %s", sc.Description)
			}
			if sc.ExternalMetadata != nil && len(sc.ExternalMetadata.Raw) > 0 {
				m := make(map[string]string)
				if err := json.Unmarshal(sc.ExternalMetadata.Raw, &m); err == nil {
					if m["field1"] == "value1" {
						foundSvcMeta1 = true
					}
				}

			}
			if len(sc.Plans) != 2 {
				t.Fatalf("Expected 2 plans for service1 but got: %d", len(sc.Plans))
			}
			for _, sp := range sc.Plans {
				if sp.Name == "s1plan2" {
					if sp.ExternalMetadata != nil && len(sp.ExternalMetadata.Raw) > 0 {
						m := make(map[string]string)
						if err := json.Unmarshal(sp.ExternalMetadata.Raw, &m); err != nil {
							t.Fatalf("Failed to unmarshal plan metadata: %s: %v", string(sp.ExternalMetadata.Raw), err)
						}
						if m["planmeta"] == "planvalue" {
							foundPlanMeta = true
						}
					}
				}
			}
		}
		// For service2 make sure we have service level metadata with three element array with elements
		// "first", "second", and "third"
		if sc.Name == "service2" {
			if sc.Description != "service 2 description" {
				t.Fatalf("Expected service2's description to be \"service 2 description\", but was: %s", sc.Description)
			}
			if sc.ExternalMetadata != nil && len(sc.ExternalMetadata.Raw) > 0 {
				m := make([]string, 0)
				if err := json.Unmarshal(sc.ExternalMetadata.Raw, &m); err != nil {
					t.Fatalf("Failed to unmarshal service metadata: %s: %v", string(sc.ExternalMetadata.Raw), err)
				}
				if len(m) != 3 {
					t.Fatalf("Expected 3 fields in metadata, but got %d", len(m))
				}
				foundFirst := false
				foundSecond := false
				foundThird := false
				for _, e := range m {
					if e == "first" {
						foundFirst = true
					}
					if e == "second" {
						foundSecond = true
					}
					if e == "third" {
						foundThird = true
					}
				}
				if !foundFirst {
					t.Fatalf("Didn't find 'first' in plan metadata")
				}
				if !foundSecond {
					t.Fatalf("Didn't find 'second' in plan metadata")
				}
				if !foundThird {
					t.Fatalf("Didn't find 'third' in plan metadata")
				}
				foundSvcMeta2 = true
			}
		}
	}
	if !foundSvcMeta1 {
		t.Fatalf("Didn't find metadata in service1")
	}
	if !foundSvcMeta2 {
		t.Fatalf("Didn't find metadata in service2")
	}
	if !foundPlanMeta {
		t.Fatalf("Didn't find metadata '' in service1 plan2")
	}

}

const testCatalogForServicePlanBindableOverride = `{
  "services": [
    {
      "name": "bindable",
      "bindable": true,
      "plans": [{
        "name": "bindable-bindable",
        "id": "s1_plan1_id"
      },
      {
        "name": "bindable-unbindable",
        "id": "s1_plan2_id",
        "bindable": false
      }]
    },
    {
      "name": "unbindable",
      "bindable": false,
      "plans": [{
        "name": "unbindable-unbindable",
        "id": "s2_plan1_id"
      },
      {
        "name": "unbindable-bindable",
        "id": "s2_plan2_id",
        "bindable": true
      }]
    }
]}`

func truePtr() *bool {
	b := true
	return &b
}

func falsePtr() *bool {
	b := false
	return &b
}

func TestCatalogConversionServicePlanBindable(t *testing.T) {
	catalog := &brokerapi.Catalog{}
	err := json.Unmarshal([]byte(testCatalogForServicePlanBindableOverride), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}

	actual, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}

	expected := []*v1alpha1.ServiceClass{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindable",
			},
			Bindable: true,
			Plans: []v1alpha1.ServicePlan{
				{
					Name:       "bindable-bindable",
					ExternalID: "s1_plan1_id",
				},
				{
					Name:       "bindable-unbindable",
					ExternalID: "s1_plan2_id",
					Bindable:   falsePtr(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "unbindable",
			},
			Bindable: false,
			Plans: []v1alpha1.ServicePlan{
				{
					Name:       "unbindable-unbindable",
					ExternalID: "s2_plan1_id",
				},
				{
					Name:       "unbindable-bindable",
					ExternalID: "s2_plan2_id",
					Bindable:   truePtr(),
				},
			},
		},
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Unexpected diff between expected and actual catalogs: %v", diff.ObjectReflectDiff(expected, actual))
	}
}

func TestIsBrokerReady(t *testing.T) {
	cases := []struct {
		name  string
		input *v1alpha1.Instance
		ready bool
	}{
		{
			name:  "ready",
			input: getTestInstanceWithStatus(v1alpha1.ConditionTrue),
			ready: true,
		},
		{
			name:  "no status",
			input: getTestInstance(),
			ready: false,
		},
		{
			name:  "not ready",
			input: getTestInstanceWithStatus(v1alpha1.ConditionFalse),
			ready: false,
		},
	}

	for _, tc := range cases {
		if e, a := tc.ready, isInstanceReady(tc.input); e != a {
			t.Errorf("%v: expected result %v, got %v", tc.name, e, a)
		}
	}
}

func TestIsPlanBindable(t *testing.T) {
	serviceClass := func(bindable bool) *v1alpha1.ServiceClass {
		serviceClass := getTestServiceClass()
		serviceClass.Bindable = bindable
		return serviceClass
	}

	servicePlan := func(bindable *bool) *v1alpha1.ServicePlan {
		return &v1alpha1.ServicePlan{
			Bindable: bindable,
		}
	}

	cases := []struct {
		name         string
		serviceClass bool
		servicePlan  *bool
		bindable     bool
	}{
		{
			name:         "service true, plan not set",
			serviceClass: true,
			bindable:     true,
		},
		{
			name:         "service true, plan false",
			serviceClass: true,
			servicePlan:  falsePtr(),
			bindable:     false,
		},
		{
			name:         "service true, plan true",
			serviceClass: true,
			servicePlan:  truePtr(),
			bindable:     true,
		},
		{
			name:         "service false, plan not set",
			serviceClass: false,
			bindable:     false,
		},
		{
			name:         "service false, plan false",
			serviceClass: false,
			servicePlan:  falsePtr(),
			bindable:     false,
		},
		{
			name:         "service false, plan true",
			serviceClass: false,
			servicePlan:  truePtr(),
			bindable:     true,
		},
	}

	for _, tc := range cases {
		sc := serviceClass(tc.serviceClass)
		plan := servicePlan(tc.servicePlan)

		if e, a := tc.bindable, isPlanBindable(sc, plan); e != a {
			t.Errorf("%v: unexpected result; expected %v, got %v", tc.name, e, a)
		}
	}
}

// newTestController creates a new test controller injected with fake clients
// and returns:
//
// - a fake kubernetes core api client
// - a fake service catalog api client
// - a fake broker catalog client
// - a fake broker instance client
// - a fake broker binding client
// - a test controller
// - the shared informers for the service catalog v1alpha1 api
//
// If there is an error, newTestController calls 'Fatal' on the injected
// testing.T.
func newTestController(t *testing.T) (
	*clientgofake.Clientset,
	*servicecatalogclientset.Clientset,
	*fakebrokerapi.Client,
	*controller,
	v1alpha1informers.Interface) {
	// create a fake kube client
	fakeKubeClient := &clientgofake.Clientset{}
	// create a fake sc client
	fakeCatalogClient := &servicecatalogclientset.Clientset{}

	catalogCl := &fakebrokerapi.CatalogClient{}
	instanceCl := fakebrokerapi.NewInstanceClient()
	bindingCl := fakebrokerapi.NewBindingClient()
	fakeBrokerClient := &fakebrokerapi.Client{
		CatalogClient:  catalogCl,
		InstanceClient: instanceCl,
		BindingClient:  bindingCl,
	}

	brokerClFunc := fakebrokerapi.NewClientFunc(catalogCl, instanceCl, bindingCl)

	// create informers
	informerFactory := servicecataloginformers.NewSharedInformerFactory(fakeCatalogClient, 0)
	serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1alpha1()

	fakeRecorder := record.NewFakeRecorder(5)

	// create a test controller
	testController, err := NewController(
		fakeKubeClient,
		fakeCatalogClient.ServicecatalogV1alpha1(),
		serviceCatalogSharedInformers.Brokers(),
		serviceCatalogSharedInformers.ServiceClasses(),
		serviceCatalogSharedInformers.Instances(),
		serviceCatalogSharedInformers.Bindings(),
		brokerClFunc,
		24*time.Hour,
		true, /* enable OSB context profile */
		fakeRecorder,
	)
	if err != nil {
		t.Fatal(err)
	}

	return fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController.(*controller), serviceCatalogSharedInformers
}

func getRecordedEvents(testController *controller) []string {
	source := testController.recorder.(*record.FakeRecorder).Events
	done := false
	events := []string{}
	for !done {
		select {
		case event := <-source:
			events = append(events, event)
		default:
			done = true
		}
	}
	return events
}

func assertNumEvents(t *testing.T, strings []string, number int) {
	if e, a := number, len(strings); e != a {
		fatalf(t, "Unexpected number of events: expected %v, got %v", e, a)
	}
}

// failfFunc is a type that defines the common signatures of T.Fatalf and
// T.Errorf.
type failfFunc func(t *testing.T, msg string, args ...interface{})

func fatalf(t *testing.T, msg string, args ...interface{}) {
	t.Log(string(debug.Stack()))
	t.Fatalf(msg, args...)
}

func errorf(t *testing.T, msg string, args ...interface{}) {
	t.Log(string(debug.Stack()))
	t.Errorf(msg, args...)
}

// assertion and expectation methods:
//
// - assertX will call t.Fatalf
// - expectX will call t.Errorf and return a boolean, allowing you to drive a 'continue'
//   in a table-type test

func assertNumberOfActions(t *testing.T, actions []clientgotesting.Action, number int) {
	testNumberOfActions(t, "" /* name */, fatalf, actions, number)
}

func expectNumberOfActions(t *testing.T, name string, actions []clientgotesting.Action, number int) bool {
	return testNumberOfActions(t, name, errorf, actions, number)
}

func testNumberOfActions(t *testing.T, name string, f failfFunc, actions []clientgotesting.Action, number int) bool {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	if e, a := number, len(actions); e != a {
		t.Logf("%+v\n", actions)
		f(t, "%vUnexpected number of actions: expected %v, got %v", logContext, e, a)
		return false
	}

	return true
}

func assertGet(t *testing.T, action clientgotesting.Action, obj interface{}) {
	assertActionFor(t, action, "get", "" /* subresource */, obj)
}

func assertCreate(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "create", "" /* subresource */, obj)
}

func assertUpdate(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "update", "" /* subresource */, obj)
}

func assertUpdateStatus(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "update", "status", obj)
}

func expectUpdateStatus(t *testing.T, name string, action clientgotesting.Action, obj interface{}) (runtime.Object, bool) {
	return testActionFor(t, name, errorf, action, "update", "status", obj)
}

func assertDelete(t *testing.T, action clientgotesting.Action, obj interface{}) {
	assertActionFor(t, action, "delete", "" /* subresource */, obj)
}

func assertActionFor(t *testing.T, action clientgotesting.Action, verb, subresource string, obj interface{}) runtime.Object {
	r, _ := testActionFor(t, "" /* name */, fatalf, action, verb, subresource, obj)
	return r
}

func testActionFor(t *testing.T, name string, f failfFunc, action clientgotesting.Action, verb, subresource string, obj interface{}) (runtime.Object, bool) {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	if e, a := verb, action.GetVerb(); e != a {
		f(t, "%vUnexpected verb: expected %v, got %v", logContext, e, a)
		return nil, false
	}

	var resource string

	switch obj.(type) {
	case *v1alpha1.Broker:
		resource = "brokers"
	case *v1alpha1.ServiceClass:
		resource = "serviceclasses"
	case *v1alpha1.Instance:
		resource = "instances"
	case *v1alpha1.Binding:
		resource = "bindings"
	}

	if e, a := resource, action.GetResource().Resource; e != a {
		f(t, "%vUnexpected resource; expected %v, got %v", logContext, e, a)
		return nil, false
	}

	if e, a := subresource, action.GetSubresource(); e != a {
		f(t, "%vUnexpected subresource; expected %v, got %v", logContext, e, a)
		return nil, false
	}

	rtObject, ok := obj.(runtime.Object)
	if !ok {
		f(t, "%vObject %+v was not a runtime.Object", logContext, obj)
		return nil, false
	}

	paramAccessor, err := metav1.ObjectMetaFor(rtObject)
	if err != nil {
		f(t, "%vError creating ObjectMetaAccessor for param object %+v: %v", logContext, rtObject, err)
		return nil, false
	}

	var (
		objectMeta   metav1.Object
		fakeRtObject runtime.Object
	)

	switch verb {
	case "get":
		getAction, ok := action.(clientgotesting.GetAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to DeleteAction", logContext, action)
			return nil, false
		}

		if e, a := paramAccessor.GetName(), getAction.GetName(); e != a {
			f(t, "%vUnexpected name: expected %v, got %v", logContext, e, a)
			return nil, false
		}

		return nil, true
	case "delete":
		deleteAction, ok := action.(clientgotesting.DeleteAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to DeleteAction", logContext, action)
			return nil, false
		}

		if e, a := paramAccessor.GetName(), deleteAction.GetName(); e != a {
			f(t, "%vUnexpected name: expected %v, got %v", logContext, e, a)
			return nil, false
		}

		return nil, true
	case "create":
		createAction, ok := action.(clientgotesting.CreateAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to CreateAction", logContext, action)
			return nil, false
		}

		fakeRtObject = createAction.GetObject()
		objectMeta, err = metav1.ObjectMetaFor(fakeRtObject)
		if err != nil {
			f(t, "%vError creating ObjectMetaAccessor for %+v", logContext, fakeRtObject)
			return nil, false
		}
	case "update":
		updateAction, ok := action.(clientgotesting.UpdateAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to UpdateAction", logContext, action)
			return nil, false
		}

		fakeRtObject = updateAction.GetObject()
		objectMeta, err = metav1.ObjectMetaFor(fakeRtObject)
		if err != nil {
			f(t, "%vError creating ObjectMetaAccessor for %+v", logContext, fakeRtObject)
			return nil, false
		}
	}

	if e, a := paramAccessor.GetName(), objectMeta.GetName(); e != a {
		f(t, "%vUnexpected name: expected %v, got %v", logContext, e, a)
		return nil, false
	}

	fakeValue := reflect.ValueOf(fakeRtObject)
	paramValue := reflect.ValueOf(obj)

	if e, a := paramValue.Type(), fakeValue.Type(); e != a {
		f(t, "%vUnexpected type of object passed to fake client; expected %v, got %v", logContext, e, a)
		return nil, false
	}

	return fakeRtObject, true
}

func assertBrokerReadyTrue(t *testing.T, obj runtime.Object) {
	assertBrokerReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertBrokerReadyFalse(t *testing.T, obj runtime.Object) {
	assertBrokerReadyCondition(t, obj, v1alpha1.ConditionFalse)
}

func assertBrokerReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus) {
	broker, ok := obj.(*v1alpha1.Broker)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.Broker", obj)
	}

	for _, condition := range broker.Status.Conditions {
		if condition.Type == v1alpha1.BrokerConditionReady && condition.Status != status {
			fatalf(t, "ready condition had unexpected status; expected %v, got %v", status, condition.Status)
		}
	}
}

func assertInstanceReadyTrue(t *testing.T, obj runtime.Object) {
	assertInstanceReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertInstanceReadyFalse(t *testing.T, obj runtime.Object, reason ...string) {
	assertInstanceReadyCondition(t, obj, v1alpha1.ConditionFalse, reason...)
}

func assertInstanceReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus, reason ...string) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}

	for _, condition := range instance.Status.Conditions {
		if condition.Type == v1alpha1.InstanceConditionReady && condition.Status != status {
			fatalf(t, "ready condition had unexpected status; expected %v, got %v", status, condition.Status)
		}
		if len(reason) == 1 && condition.Reason != reason[0] {
			fatalf(t, "unexpected reason; expected %v, got %v", reason[0], condition.Reason)
		}
	}
}

func assertAsyncOpInProgressTrue(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if !instance.Status.AsyncOpInProgress {
		t.Fatalf("expected AsyncOpInProgress to be true but was %v", instance.Status.AsyncOpInProgress)
	}
}

func assertAsyncOpInProgressFalse(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if instance.Status.AsyncOpInProgress {
		t.Fatalf("expected AsyncOpInProgress to be false but was %v", instance.Status.AsyncOpInProgress)
	}
}

func assertInstanceLastOperation(t *testing.T, obj runtime.Object, operation string) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if instance.Status.LastOperation == nil {
		if operation != "" {
			t.Fatalf("Last Operation <nil> is not what was expected: %q", operation)
		}
	} else if *instance.Status.LastOperation != operation {
		t.Fatalf("Last Operation %q is not what was expected: %q", *instance.Status.LastOperation, operation)
	}
}

func assertInstanceDashboardURL(t *testing.T, obj runtime.Object, dashboardURL string) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if instance.Status.DashboardURL == nil {
		t.Fatal("DashboardURL was nil")
	} else if *instance.Status.DashboardURL != dashboardURL {
		t.Fatalf("Unexpected DashboardURL: expected %q, got %q", dashboardURL, *instance.Status.DashboardURL)
	}
}

func assertBindingReadyTrue(t *testing.T, obj runtime.Object) {
	assertBindingReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertBindingReadyFalse(t *testing.T, obj runtime.Object, reason ...string) {
	assertBindingReadyCondition(t, obj, v1alpha1.ConditionFalse, reason...)
}

func assertBindingReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus, reason ...string) {
	binding, ok := obj.(*v1alpha1.Binding)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.Binding", obj)
	}

	for _, condition := range binding.Status.Conditions {
		if condition.Type == v1alpha1.BindingConditionReady && condition.Status != status {
			t.Logf("ready condition: %+v", condition)
			fatalf(t, "ready condition had unexpected status; expected %v, got %v", status, condition.Status)
		}
		if len(reason) == 1 && condition.Reason != reason[0] {
			fatalf(t, "unexpected reason; expected %v, got %v", reason[0], condition.Reason)
		}
	}
}

func assertEmptyFinalizers(t *testing.T, obj runtime.Object) {
	accessor, err := metav1.ObjectMetaFor(obj)
	if err != nil {
		fatalf(t, "Error creating ObjectMetaAccessor for param object %+v: %v", obj, err)
	}

	if len(accessor.GetFinalizers()) != 0 {
		fatalf(t, "Unexpected number of finalizers; expected 0, got %v", len(accessor.GetFinalizers()))
	}
}
