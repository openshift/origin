package controller

import (
	"testing"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/project/api"
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/util"
)

func TestSyncNamespaceThatIsTerminating(t *testing.T) {
	mockKubeClient := &ktestclient.Fake{}
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
		ktestclient.NewListAction("buildconfigs", "", nil, nil),
		ktestclient.NewListAction("policies", "", nil, nil),
		ktestclient.NewListAction("imagestreams", "", nil, nil),
		ktestclient.NewListAction("policybindings", "", nil, nil),
		ktestclient.NewListAction("rolebindings", "", nil, nil),
		ktestclient.NewListAction("roles", "", nil, nil),
		ktestclient.NewListAction("routes", "", nil, nil),
		ktestclient.NewListAction("templates", "", nil, nil),
		ktestclient.NewListAction("builds", "", nil, nil),
		ktestclient.NewListAction("namespace", "", nil, nil),
		ktestclient.NewListAction("deploymentconfig", "", nil, nil),
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
	mockKubeClient := &ktestclient.Fake{}
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
