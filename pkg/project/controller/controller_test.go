package controller

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

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
	expectedActionSet := []core.Action{
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "buildconfigs"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "policies"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "imagestreams"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "policybindings"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "rolebindings"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "roles"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "routes"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "templates"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "builds"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "namespace"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "deploymentconfig"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "egressnetworkpolicy"}, "", kapi.ListOptions{}),
		core.NewListAction(unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "rolebindingrestrictions"}, "", kapi.ListOptions{}),
	}
	kubeActionSet := []core.Action{}
	originActionSet := []core.Action{}
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
	originActionSet := []core.Action{}
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
