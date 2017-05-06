package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/project/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func TestSyncNamespaceThatIsTerminating(t *testing.T) {
	mockKubeClient := &fake.Clientset{}
	nm := &ProjectFinalizerController{
		client: mockKubeClient,
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
	err := nm.finalize(testNamespace)
	if err != nil {
		t.Errorf("Unexpected error when handling namespace %v", err)
	}

	// TODO: we will expect a finalize namespace call after rebase
	expectedActionSet := []clientgotesting.Action{
		clientgotesting.NewListAction(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespace"}, "", metav1.ListOptions{}),
	}
	kubeActionSet := []clientgotesting.Action{}
	for i := range mockKubeClient.Actions() {
		kubeActionSet = append(kubeActionSet, mockKubeClient.Actions()[i])
	}

	if (len(kubeActionSet)) != len(expectedActionSet) {
		t.Errorf("Expected actions: %v, but got: %v", expectedActionSet, kubeActionSet)
	}
}

func TestSyncNamespaceThatIsActive(t *testing.T) {
	mockKubeClient := &fake.Clientset{}
	nm := &ProjectFinalizerController{
		client: mockKubeClient,
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
	err := nm.finalize(testNamespace)
	if err != nil {
		t.Errorf("Unexpected error when handling namespace %v", err)
	}
	kubeActionSet := []clientgotesting.Action{}
	for i := range mockKubeClient.Actions() {
		kubeActionSet = append(kubeActionSet, mockKubeClient.Actions()[i])
	}

	if (len(kubeActionSet)) != 0 {
		t.Errorf("Expected no actions from contoller, but got: %#v", kubeActionSet)
	}
}
