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

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"

	"strings"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	clientgotesting "k8s.io/client-go/testing"
)

// TestShouldReconcileClusterServiceBroker ensures that with the expected conditions the
// reconciler is reported as needing to run.
//
// The test cases are proving:
// - broker without ready condition will reconcile
// - broker with deletion timestamp set will reconcile
// - broker without ready condition, with status will reconcile
// - broker without ready condition, without status will reconcile
// - broker with status/ready, past relist interval will reconcile
// - broker with status/ready, within relist interval will NOT reconcile
// - broker with status/ready/checksum, will reconcile
func TestShouldReconcileClusterServiceBroker(t *testing.T) {
	// Anonymous struct fields:
	// name: short description of the test
	// broker: broker object to test
	// now: what time the interval is calculated with respect to interval
	// reconcile: whether or not the reconciler should run, the return of
	// shouldReconcileClusterServiceBroker
	cases := []struct {
		name      string
		broker    *v1beta1.ClusterServiceBroker
		now       time.Time
		reconcile bool
		err       error
	}{
		{
			name: "no status",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBroker()
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Minute}
				return broker
			}(),
			now:       time.Now(),
			reconcile: true,
		},
		{
			name: "deletionTimestamp set",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue)
				broker.DeletionTimestamp = &metav1.Time{}
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Hour}
				return broker
			}(),
			now:       time.Now(),
			reconcile: true,
		},
		{
			name: "no ready condition",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBroker()
				broker.Status = v1beta1.ClusterServiceBrokerStatus{
					Conditions: []v1beta1.ServiceBrokerCondition{
						{
							Type:   v1beta1.ServiceBrokerConditionType("NotARealCondition"),
							Status: v1beta1.ConditionTrue,
						},
					},
				}
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Minute}
				return broker
			}(),
			now:       time.Now(),
			reconcile: true,
		},
		{
			name: "not ready",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionFalse)
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Minute}
				return broker
			}(),
			now:       time.Now(),
			reconcile: true,
		},
		{
			name: "ready, interval elapsed",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue)
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Minute}
				return broker
			}(),
			now:       time.Now(),
			reconcile: true,
		},
		{
			name: "ready, interval not elapsed",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue)
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Hour}
				return broker
			}(),
			now:       time.Now(),
			reconcile: false,
		},
		{
			name: "ready, interval not elapsed, spec changed",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue)
				broker.Generation = 2
				broker.Status.ReconciledGeneration = 1
				broker.Spec.RelistDuration = &metav1.Duration{Duration: 3 * time.Hour}
				return broker
			}(),
			now:       time.Now(),
			reconcile: true,
		},
		{
			name: "ready, duration behavior, nil duration",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue)
				broker.Spec.RelistBehavior = v1beta1.ServiceBrokerRelistBehaviorDuration
				broker.Spec.RelistDuration = nil
				return broker
			}(),
			now:       time.Now(),
			reconcile: false,
		},
		{
			name: "ready, manual behavior",
			broker: func() *v1beta1.ClusterServiceBroker {
				broker := getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue)
				broker.Spec.RelistBehavior = v1beta1.ServiceBrokerRelistBehaviorManual
				return broker
			}(),
			now:       time.Now(),
			reconcile: false,
		},
	}

	for _, tc := range cases {
		var ltt *time.Time
		if len(tc.broker.Status.Conditions) != 0 {
			ltt = &tc.broker.Status.Conditions[0].LastTransitionTime.Time
		}

		if tc.broker.Spec.RelistDuration != nil {
			interval := tc.broker.Spec.RelistDuration.Duration
			t.Logf("%v: now: %v, interval: %v, last transition time: %v", tc.name, tc.now, interval, ltt)
		} else {
			t.Logf("broker.Spec.RelistDuration set to nil")
		}

		actual := shouldReconcileClusterServiceBroker(tc.broker, tc.now)

		if e, a := tc.reconcile, actual; e != a {
			t.Errorf("%v: unexpected result: expected %v, got %v", tc.name, e, a)
		}
	}
}

// TestReconcileClusterServiceBrokerExistingServiceClassAndServicePlan
// verifies a simple, successful run of reconcileClusterServiceBroker() when a
// ClusterServiceClass and plan already exist.  This test will cause
// reconcileBroker() to fetch the catalog from the ClusterServiceBroker,
// create a Service Class for the single service that it lists and reconcile
// the service class ensuring the name and id of the relisted service matches
// the existing entry and updates the service catalog. There will be two
// additional reconciles of plans before the final broker update
func TestReconcileClusterServiceBrokerExistingServiceClassAndServicePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()
	testClusterServicePlan := getTestClusterServicePlan()
	testClusterServicePlanNonbindable := getTestClusterServicePlanNonbindable()
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(testClusterServiceClass)

	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{
			Items: []v1beta1.ClusterServiceClass{
				*testClusterServiceClass,
			},
		}, nil
	})

	if err := testController.reconcileClusterServiceBroker(getTestClusterServiceBroker()); err != nil {
		t.Fatalf("This should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", "test-broker"),
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 6)
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertCreate(t, actions[2], testClusterServicePlan)
	assertCreate(t, actions[3], testClusterServicePlanNonbindable)
	assertUpdate(t, actions[4], testClusterServiceClass)

	// 4 update action for broker status subresource
	updatedClusterServiceBroker := assertUpdateStatus(t, actions[5], getTestClusterServiceBroker())
	assertClusterServiceBrokerReadyTrue(t, updatedClusterServiceBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

func TestReconcileClusterServiceBrokerRemovedClusterServiceClass(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()
	testRemovedClusterServiceClass := getTestRemovedClusterServiceClass()
	testClusterServicePlan := getTestClusterServicePlan()
	testClusterServicePlanNonbindable := getTestClusterServicePlanNonbindable()
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(testClusterServiceClass)
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(testRemovedClusterServiceClass)

	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{
			Items: []v1beta1.ClusterServiceClass{
				*testClusterServiceClass,
				*testRemovedClusterServiceClass,
			},
		}, nil
	})

	if err := testController.reconcileClusterServiceBroker(getTestClusterServiceBroker()); err != nil {
		t.Fatalf("This should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", "test-broker"),
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 7)
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertCreate(t, actions[2], testClusterServicePlan)
	assertCreate(t, actions[3], testClusterServicePlanNonbindable)
	assertUpdate(t, actions[4], testClusterServiceClass)
	assertUpdateStatus(t, actions[5], testRemovedClusterServiceClass)

	updatedClusterServiceBroker := assertUpdateStatus(t, actions[6], getTestClusterServiceBroker())
	assertClusterServiceBrokerReadyTrue(t, updatedClusterServiceBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

func TestReconcileClusterServiceBrokerRemovedClusterServicePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()
	testClusterServicePlan := getTestClusterServicePlan()
	testClusterServicePlanNonbindable := getTestClusterServicePlanNonbindable()
	testRemovedClusterServicePlan := getTestRemovedClusterServicePlan()
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(testClusterServiceClass)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(testRemovedClusterServicePlan)

	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{
			Items: []v1beta1.ClusterServiceClass{
				*testClusterServiceClass,
			},
		}, nil
	})
	fakeCatalogClient.AddReactor("list", "clusterserviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServicePlanList{
			Items: []v1beta1.ClusterServicePlan{
				*testRemovedClusterServicePlan,
			},
		}, nil
	})

	if err := testController.reconcileClusterServiceBroker(getTestClusterServiceBroker()); err != nil {
		t.Fatalf("This should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", "test-broker"),
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 7)
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertCreate(t, actions[2], testClusterServicePlan)
	assertCreate(t, actions[3], testClusterServicePlanNonbindable)
	assertUpdateStatus(t, actions[4], testRemovedClusterServicePlan)
	assertUpdate(t, actions[5], testClusterServiceClass)

	updatedClusterServiceBroker := assertUpdateStatus(t, actions[6], getTestClusterServiceBroker())
	assertClusterServiceBrokerReadyTrue(t, updatedClusterServiceBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestReconcileClusterServiceBrokerExistingClusterServiceClassDifferentBroker simulates catalog
// refresh where broker lists a service which matches an existing, already
// cataloged service but the service points to a different ClusterServiceBroker.  Results in an error.
func TestReconcileClusterServiceBrokerExistingClusterServiceClassDifferentBroker(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()
	testClusterServiceClass.Spec.ClusterServiceBrokerName = "notTheSame"

	testClusterServicePlan := getTestClusterServicePlan()
	testClusterServicePlan.Spec.ClusterServiceBrokerName = "notTheSame"

	testClusterServicePlanNonbindable := getTestClusterServicePlanNonbindable()
	testClusterServicePlanNonbindable.Spec.ClusterServiceBrokerName = "notTheSame"

	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(testClusterServiceClass)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(testClusterServicePlan)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(testClusterServicePlanNonbindable)

	if err := testController.reconcileClusterServiceBroker(getTestClusterServiceBroker()); err == nil {
		t.Fatal("The same service class should not belong to two different brokers.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 3)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", "test-broker"),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	updatedClusterServiceBroker := assertUpdateStatus(t, actions[2], getTestClusterServiceBroker())
	assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeWarning + " " + errorSyncingCatalogReason + ` Error reconciling ClusterServicePlan (K8S: "PGUID" ExternalName: "test-plan"): ClusterServicePlan (K8S: "PGUID" ExternalName: "test-plan") already exists for Broker "notTheSame"`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event; expected\n%v, got\n%v", e, a)
	}
}

// TestReconcileClusterServiceBrokerDelete simulates a broker reconciliation where broker was marked for deletion.
// Results in service class and broker both being deleted.
func TestReconcileClusterServiceBrokerDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()
	testClusterServicePlan := getTestClusterServicePlan()

	broker := getTestClusterServiceBroker()
	broker.DeletionTimestamp = &metav1.Time{}
	broker.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	fakeCatalogClient.AddReactor("get", "clusterservicebrokers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, broker, nil
	})
	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{
			Items: []v1beta1.ClusterServiceClass{
				*testClusterServiceClass,
			},
		}, nil
	})
	fakeCatalogClient.AddReactor("list", "clusterserviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServicePlanList{
			Items: []v1beta1.ClusterServicePlan{
				*testClusterServicePlan,
			},
		}, nil
	})

	err := testController.reconcileClusterServiceBroker(broker)
	if err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	// The four actions should be:
	// - list serviceplans
	// - delete serviceplans
	// - list serviceclasses
	// - delete serviceclass
	// - update the ready condition
	// - get the broker
	// - remove the finalizer
	assertNumberOfActions(t, actions, 7)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", broker.Name),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertDelete(t, actions[2], testClusterServicePlan)
	assertDelete(t, actions[3], testClusterServiceClass)
	updatedClusterServiceBroker := assertUpdateStatus(t, actions[4], broker)
	assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)

	assertGet(t, actions[5], broker)

	updatedClusterServiceBroker = assertUpdateStatus(t, actions[6], broker)
	assertEmptyFinalizers(t, updatedClusterServiceBroker)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeNormal + " " + successClusterServiceBrokerDeletedReason + " " + "The broker test-broker was deleted successfully."
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileClusterServiceBrokerErrorFetchingCatalog simulates broker reconciliation where
// OSB client responds with an error for getting the catalog which in turn causes
// reconcileClusterServiceBroker() to return an error.
func TestReconcileClusterServiceBrokerErrorFetchingCatalog(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Error: errors.New("ooops"),
		},
	})

	broker := getTestClusterServiceBroker()

	if err := testController.reconcileClusterServiceBroker(broker); err == nil {
		t.Fatal("Should have failed to get the catalog.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedClusterServiceBroker := assertUpdateStatus(t, actions[0], broker)
	assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)

	updatedClusterServiceBroker = assertUpdateStatus(t, actions[1], broker)
	assertClusterServiceBrokerOperationStartTimeSet(t, updatedClusterServiceBroker, true)

	assertNumberOfActions(t, fakeKubeClient.Actions(), 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeWarning + " " + errorFetchingCatalogReason + " " + `ClusterServiceBroker "test-broker": Error getting broker catalog: ooops`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileClusterServiceBrokerZeroServices simulates broker reconciliation where
// OSB client responds with zero services which causes reconcileClusterServiceBroker()
// to return an error
func TestReconcileClusterServiceBrokerZeroServices(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: &osb.CatalogResponse{},
		},
	})

	broker := getTestClusterServiceBroker()

	if err := testController.reconcileClusterServiceBroker(broker); err == nil {
		t.Fatal("ClusterServiceBroker should not have had any Service Classes.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedClusterServiceBroker := assertUpdateStatus(t, actions[0], broker)
	assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)

	assertNumberOfActions(t, fakeKubeClient.Actions(), 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeWarning + " " + errorSyncingCatalogReason + ` Error getting catalog payload for broker "test-broker"; received zero services; at least one service is required`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event; \nexpected: %v\ngot:     %v", e, a)
	}
}

func TestReconcileClusterServiceBrokerWithAuth(t *testing.T) {
	basicAuthInfo := &v1beta1.ServiceBrokerAuthInfo{
		Basic: &v1beta1.BasicAuthConfig{
			SecretRef: &v1.ObjectReference{
				Namespace: "test-ns",
				Name:      "auth-secret",
			},
		},
	}
	bearerAuthInfo := &v1beta1.ServiceBrokerAuthInfo{
		Bearer: &v1beta1.BearerTokenAuthConfig{
			SecretRef: &v1.ObjectReference{
				Namespace: "test-ns",
				Name:      "auth-secret",
			},
		},
	}
	basicAuthSecret := &v1.Secret{
		Data: map[string][]byte{
			v1beta1.BasicAuthUsernameKey: []byte("foo"),
			v1beta1.BasicAuthPasswordKey: []byte("bar"),
		},
	}
	bearerAuthSecret := &v1.Secret{
		Data: map[string][]byte{
			v1beta1.BearerTokenKey: []byte("token"),
		},
	}

	// The test cases here are testing the correctness of authentication with broker
	//
	// Anonymous struct fields:
	// name: short description of the test
	// authInfo: broker auth configuration
	// secret: auth secret to be returned upon request from Service Catalog
	// shouldSucceed: whether authentication should succeed
	cases := []struct {
		name          string
		authInfo      *v1beta1.ServiceBrokerAuthInfo
		secret        *v1.Secret
		shouldSucceed bool
	}{
		{
			name:          "basic auth - normal",
			authInfo:      basicAuthInfo,
			secret:        basicAuthSecret,
			shouldSucceed: true,
		},
		{
			name:          "basic auth - invalid secret",
			authInfo:      basicAuthInfo,
			secret:        bearerAuthSecret,
			shouldSucceed: false,
		},
		{
			name:          "basic auth - secret not found",
			authInfo:      basicAuthInfo,
			secret:        nil,
			shouldSucceed: false,
		},
		{
			name:          "bearer auth - normal",
			authInfo:      bearerAuthInfo,
			secret:        bearerAuthSecret,
			shouldSucceed: true,
		},
		{
			name:          "bearer auth - invalid secret",
			authInfo:      bearerAuthInfo,
			secret:        basicAuthSecret,
			shouldSucceed: false,
		},
		{
			name:          "bearer auth - secret not found",
			authInfo:      bearerAuthInfo,
			secret:        nil,
			shouldSucceed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testReconcileClusterServiceBrokerWithAuth(t, tc.authInfo, tc.secret, tc.shouldSucceed)
		})
	}
}

func testReconcileClusterServiceBrokerWithAuth(t *testing.T, authInfo *v1beta1.ServiceBrokerAuthInfo, secret *v1.Secret, shouldSucceed bool) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, fakeosb.FakeClientConfiguration{})

	broker := getTestClusterServiceBrokerWithAuth(authInfo)
	if secret != nil {
		addGetSecretReaction(fakeKubeClient, secret)
	} else {
		addGetSecretNotFoundReaction(fakeKubeClient)
	}
	testClusterServiceClass := getTestClusterServiceClass()
	fakeClusterServiceBrokerClient.CatalogReaction = &fakeosb.CatalogReaction{
		Response: &osb.CatalogResponse{
			Services: []osb.Service{
				{
					ID:   testClusterServiceClass.Spec.ExternalID,
					Name: testClusterServiceClass.Name,
				},
			},
		},
	}

	err := testController.reconcileClusterServiceBroker(broker)
	if shouldSucceed && err != nil {
		t.Fatal("Should have succeeded to get the catalog for the broker. got error: ", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	if shouldSucceed {
		// GetCatalog
		assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
		assertGetCatalog(t, brokerActions[0])
	} else {
		assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)
	}

	actions := fakeCatalogClient.Actions()
	if shouldSucceed {
		assertNumberOfActions(t, actions, 2)
		assertCreate(t, actions[0], testClusterServiceClass)
		updatedClusterServiceBroker := assertUpdateStatus(t, actions[1], broker)
		assertClusterServiceBrokerReadyTrue(t, updatedClusterServiceBroker)
	} else {
		assertNumberOfActions(t, actions, 1)
		updatedClusterServiceBroker := assertUpdateStatus(t, actions[0], broker)
		assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)
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

	var expectedEvent string
	if shouldSucceed {
		expectedEvent = corev1.EventTypeNormal + " " + successFetchedCatalogReason + " " + successFetchedCatalogMessage
	} else {
		expectedEvent = corev1.EventTypeWarning + " " + errorAuthCredentialsReason + " " + `ClusterServiceBroker "test-broker": Error getting broker auth credentials`
	}
	if e, a := expectedEvent, events[0]; !strings.HasPrefix(a, e) {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileClusterServiceBrokerWithReconcileError simulates broker reconciliation where
// creation of a service class causes an error which causes ReconcileClusterServiceBroker to
// return an error
func TestReconcileClusterServiceBrokerWithReconcileError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, getTestCatalogConfig())

	broker := getTestClusterServiceBroker()

	fakeCatalogClient.AddReactor("create", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("error creating serviceclass")
	})

	if err := testController.reconcileClusterServiceBroker(broker); err == nil {
		t.Fatal("There should have been an error.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 6)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", broker.Name),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertCreate(t, actions[2], getTestClusterServicePlan())
	assertCreate(t, actions[3], getTestClusterServicePlanNonbindable())

	// the two plans in the catalog as two separate actions

	createSCAction := actions[4].(clientgotesting.CreateAction)
	createdSC, ok := createSCAction.GetObject().(*v1beta1.ClusterServiceClass)
	if !ok {
		t.Fatalf("couldn't convert to a ClusterServiceClass: %+v", createSCAction.GetObject())
	}
	if e, a := getTestClusterServiceClass(), createdSC; !reflect.DeepEqual(e, a) {
		t.Fatalf("unexpected diff for created ClusterServiceClass: %v,\n\nEXPECTED: %+v\n\nACTUAL:  %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
	updatedClusterServiceBroker := assertUpdateStatus(t, actions[5], broker)
	assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeWarning + " " + errorSyncingCatalogReason + ` Error reconciling ClusterServiceClass (K8S: "SCGUID" ExternalName: "test-serviceclass") (broker "test-broker"): error creating serviceclass`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileClusterServiceBrokerSuccessOnFinalRetry verifies that reconciliation can
// succeed on the last attempt before timing out of the retry loop
func TestReconcileClusterServiceBrokerSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()

	broker := getTestClusterServiceBroker()
	// seven days ago, before the last refresh period
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	broker.Status.OperationStartTime = &startTime

	if err := testController.reconcileClusterServiceBroker(broker); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 7)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", broker.Name),
	}

	// first action should be an update action to clear OperationStartTime
	updatedClusterServiceBroker := assertUpdateStatus(t, actions[0], getTestClusterServiceBroker())
	assertClusterServiceBrokerOperationStartTimeSet(t, updatedClusterServiceBroker, false)

	assertList(t, actions[1], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[2], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertCreate(t, actions[3], getTestClusterServicePlan())
	assertCreate(t, actions[4], getTestClusterServicePlanNonbindable())
	assertCreate(t, actions[5], testClusterServiceClass)

	updatedClusterServiceBroker = assertUpdateStatus(t, actions[6], getTestClusterServiceBroker())
	assertClusterServiceBrokerReadyTrue(t, updatedClusterServiceBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestReconcileClusterServiceBrokerFailureOnFinalRetry verifies that reconciliation
// completes in the event of an error after the retry duration elapses.
func TestReconcileClusterServiceBrokerFailureOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Error: errors.New("ooops"),
		},
	})

	broker := getTestClusterServiceBroker()
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	broker.Status.OperationStartTime = &startTime

	if err := testController.reconcileClusterServiceBroker(broker); err != nil {
		t.Fatalf("Should have return no error because the retry duration has elapsed: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedClusterServiceBroker := assertUpdateStatus(t, actions[0], broker)
	assertClusterServiceBrokerReadyFalse(t, updatedClusterServiceBroker)

	updatedClusterServiceBroker = assertUpdateStatus(t, actions[1], broker)
	assertClusterServiceBrokerCondition(t, updatedClusterServiceBroker, v1beta1.ServiceBrokerConditionFailed, v1beta1.ConditionTrue)
	assertClusterServiceBrokerOperationStartTimeSet(t, updatedClusterServiceBroker, false)

	assertNumberOfActions(t, fakeKubeClient.Actions(), 0)

	expectedEventPrefixes := []string{
		corev1.EventTypeWarning + " " + errorFetchingCatalogReason,
		corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason,
	}
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, len(expectedEventPrefixes))

	for i, e := range expectedEventPrefixes {
		a := events[i]
		if !strings.HasPrefix(a, e) {
			t.Fatalf("Received unexpected event:\n  expected prefix: %v\n  got: %v", e, a)
		}
	}
}

// TestReconcileClusterServiceBrokerWithStatusUpdateError verifies that the reconciler
// returns an error when there is a conflict updating the status of the resource.
// This is an otherwise successful scenario where the update to set the
// ready condition fails.
func TestReconcileClusterServiceBrokerWithStatusUpdateError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, getTestCatalogConfig())

	testClusterServiceClass := getTestClusterServiceClass()

	broker := getTestClusterServiceBroker()

	fakeCatalogClient.AddReactor("update", "clusterservicebrokers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update error")
	})

	err := testController.reconcileClusterServiceBroker(broker)
	if err == nil {
		t.Fatalf("expected error from but got none")
	}
	if e, a := "update error", err.Error(); e != a {
		t.Fatalf("unexpected error returned: expected %q, got %q", e, a)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertGetCatalog(t, brokerActions[0])

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 6)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.clusterServiceBrokerName", broker.Name),
	}

	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)
	assertCreate(t, actions[2], getTestClusterServicePlan())
	assertCreate(t, actions[3], getTestClusterServicePlanNonbindable())
	assertCreate(t, actions[4], testClusterServiceClass)

	// 4 update action for broker status subresource
	updatedClusterServiceBroker := assertUpdateStatus(t, actions[5], getTestClusterServiceBroker())
	assertClusterServiceBrokerReadyTrue(t, updatedClusterServiceBroker)

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestUpdateServiceBrokerCondition ensures that with specific conditions
// the broker correctly reflects the changes during updateServiceBrokerCondition().
//
// The test cases are proving:
// - broker transitions from unset status to not ready results in status change and new time
// - broker transitions from not ready to not ready results in no changes
// - broker transitions from not ready to not ready and with reason & msg updates results in no time change, but reflects new reason & msg
// - broker transitions from not ready to ready results in status change & new time
// - broker transitions from ready to ready results in no status change
// - broker transitions from ready to not ready results in status change & new time
// - condition reason & message should always be updated
func TestUpdateServiceBrokerCondition(t *testing.T) {
	// Anonymous struct fields:
	// name: short description of the test
	// input: broker object to test
	// status: new condition status
	// reason: condition reason
	// message: condition message
	// transitionTimeChanged: true if the test conditions should result in transition time change
	cases := []struct {
		name                  string
		input                 *v1beta1.ClusterServiceBroker
		status                v1beta1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestClusterServiceBroker(),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestClusterServiceBrokerWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready with reason and message change",
			input:                 getTestClusterServiceBrokerWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestClusterServiceBrokerWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionTrue,
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionTrue,
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestClusterServiceBrokerWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, _, testController, _ := newTestController(t, getTestCatalogConfig())

		clone, err := api.Scheme.DeepCopy(tc.input)
		if err != nil {
			t.Errorf("%v: deep copy failed", tc.name)
			continue
		}

		inputClone := clone.(*v1beta1.ClusterServiceBroker)

		err = testController.updateClusterServiceBrokerCondition(tc.input, v1beta1.ServiceBrokerConditionReady, tc.status, tc.reason, tc.message)
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

		updatedClusterServiceBroker, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedClusterServiceBroker.(*v1beta1.ClusterServiceBroker)
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
			t.Errorf("%v: condition message didn't match; expected %v, got %v", tc.name, e, a)
		}
	}
}
