package controller

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/project/api"
	"k8s.io/kubernetes/pkg/client/testing/fake"
)

func TestSyncNamespaceThatIsTerminating(t *testing.T) {
	mockKubeClient := &fake.Clientset{}
	mockOriginClient := &testclient.Fake{}
	nm := NamespaceController{
		KubeClient: mockKubeClient,
		Client:     mockOriginClient,
	}
	now := unversioned.Now()
	testNamespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:              "test",
			ResourceVersion:   "1",
			DeletionTimestamp: &now,
		},
		Spec: kapi.NamespaceSpec{
			Finalizers: []kapi.FinalizerName{kapi.FinalizerKubernetes, api.FinalizerOrigin},
		},
		Status: kapi.NamespaceStatus{
			Phase: kapi.NamespaceTerminating,
		},
	}
	err := nm.Handle(testNamespace)
	if err != nil {
		t.Errorf("Unexpected error when handling namespace %v", err)
	}

	// TODO: we will expect a finalize namespace call after rebase
	expectedActionSet := []ktestclient.Action{
		ktestclient.NewListAction("buildconfigs", "", kapi.ListOptions{}),
		ktestclient.NewListAction("policies", "", kapi.ListOptions{}),
		ktestclient.NewListAction("imagestreams", "", kapi.ListOptions{}),
		ktestclient.NewListAction("policybindings", "", kapi.ListOptions{}),
		ktestclient.NewListAction("rolebindings", "", kapi.ListOptions{}),
		ktestclient.NewListAction("roles", "", kapi.ListOptions{}),
		ktestclient.NewListAction("routes", "", kapi.ListOptions{}),
		ktestclient.NewListAction("templates", "", kapi.ListOptions{}),
		ktestclient.NewListAction("builds", "", kapi.ListOptions{}),
		ktestclient.NewListAction("namespace", "", kapi.ListOptions{}),
		ktestclient.NewListAction("deploymentconfig", "", kapi.ListOptions{}),
	}
	actionSet := []ktestclient.Action{}
	for i := range mockKubeClient.Actions() {
		actionSet = append(actionSet, mockKubeClient.Actions()[i])
	}
	for i := range mockOriginClient.Actions() {
		actionSet = append(actionSet, mockOriginClient.Actions()[i])
	}

	if len(actionSet) != len(expectedActionSet) {
		t.Errorf("Expected actions: %v, but got: %v", expectedActionSet, actionSet)
	}
}

func TestSyncNamespaceThatIsActive(t *testing.T) {
	mockKubeClient := &fake.Clientset{}
	mockOriginClient := &testclient.Fake{}
	nm := NamespaceController{
		KubeClient: mockKubeClient,
		Client:     mockOriginClient,
	}
	testNamespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
		Spec: kapi.NamespaceSpec{
			Finalizers: []kapi.FinalizerName{kapi.FinalizerKubernetes, api.FinalizerOrigin},
		},
		Status: kapi.NamespaceStatus{
			Phase: kapi.NamespaceActive,
		},
	}
	err := nm.Handle(testNamespace)
	if err != nil {
		t.Errorf("Unexpected error when handling namespace %v", err)
	}
	actionSet := []ktestclient.Action{}
	for i := range mockKubeClient.Actions() {
		actionSet = append(actionSet, mockKubeClient.Actions()[i])
	}
	for i := range mockOriginClient.Actions() {
		actionSet = append(actionSet, mockOriginClient.Actions()[i])
	}

	if len(actionSet) != 0 {
		t.Errorf("Expected no action from controller, but got: %v", actionSet)
	}
}
