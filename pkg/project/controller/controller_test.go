package controller

import (
	"testing"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"
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
	now := metav1.Now()
	testNamespace := &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
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
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "buildconfigs"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "policies"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "imagestreams"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "policybindings"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "rolebindings"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "roles"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "routes"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "templates"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "builds"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespace"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "deploymentconfig"}, "", metainternal.ListOptions{}),
		core.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "egressnetworkpolicy"}, "", metainternal.ListOptions{}),
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
		ObjectMeta: metav1.ObjectMeta{
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
