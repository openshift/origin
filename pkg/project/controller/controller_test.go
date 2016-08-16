package controller

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/project/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
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
		ktestclient.NewListAction("egressnetworkpolicy", "", kapi.ListOptions{}),
	}
	kubeActionSet := []core.Action{}
	originActionSet := []ktestclient.Action{}
	for i := range mockKubeClient.Actions() {
		kubeActionSet = append(kubeActionSet, mockKubeClient.Actions()[i])
	}
	for i := range mockOriginClient.Actions() {
		originActionSet = append(originActionSet, mockOriginClient.Actions()[i])
	}

	if (len(kubeActionSet) + len(originActionSet)) != len(expectedActionSet) {
		t.Errorf("Expected actions: %v, but got: %v and %v", expectedActionSet, originActionSet, kubeActionSet)
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
	kubeActionSet := []core.Action{}
	originActionSet := []ktestclient.Action{}
	for i := range mockKubeClient.Actions() {
		kubeActionSet = append(kubeActionSet, mockKubeClient.Actions()[i])
	}
	for i := range mockOriginClient.Actions() {
		originActionSet = append(originActionSet, mockOriginClient.Actions()[i])
	}

	if (len(kubeActionSet) + len(originActionSet)) != 0 {
		t.Errorf("Expected no actions from contoller, but got: %#v and %#v", originActionSet, kubeActionSet)
	}
}
