package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

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
	expectedActionSet := []clientgotesting.Action{
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "buildconfigs"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "policies"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "imagestreams"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "policybindings"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "rolebindings"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "roles"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "routes"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "templates"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "builds"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespace"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "deploymentconfig"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "egressnetworkpolicy"}, "", metav1.ListOptions{}),
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "rolebindingrestrictions"}, "", metav1.ListOptions{}),
	}
	kubeActionSet := []clientgotesting.Action{}
	originActionSet := []clientgotesting.Action{}
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
	kubeActionSet := []clientgotesting.Action{}
	originActionSet := []clientgotesting.Action{}
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
