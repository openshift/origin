package events

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func fakeControllerRef(t *testing.T) *corev1.ObjectReference {
	podNameEnvFunc = func() string {
		return "test"
	}
	client := fake.NewSimpleClientset(fakePod("test-namespace", "test"), fakeReplicaSet("test-namespace", "test"))

	ref, err := GetControllerReferenceForCurrentPod(client, "test-namespace", nil)
	if err != nil {
		t.Fatalf("unable to get object reference: %v", err)
	}
	return ref
}

func fakePod(namespace, name string) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = name
	pod.Namespace = namespace
	truePtr := true
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         "apps/v1",
			Kind:               "ReplicaSet",
			Name:               "test",
			UID:                "05022234-d394-11e8-8169-42010a8e0003",
			Controller:         &truePtr,
			BlockOwnerDeletion: &truePtr,
		},
	})
	return pod
}

func fakeReplicaSet(namespace, name string) *appsv1.ReplicaSet {
	rs := &appsv1.ReplicaSet{}
	rs.Name = name
	rs.Namespace = namespace
	truePtr := true
	rs.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         "apps/v1",
			Kind:               "Deployment",
			Name:               "test",
			UID:                "15022234-d394-11e8-8169-42010a8e0003",
			Controller:         &truePtr,
			BlockOwnerDeletion: &truePtr,
		},
	})
	return rs
}

func TestRecorder(t *testing.T) {
	client := fake.NewSimpleClientset()
	r := NewRecorder(client.CoreV1().Events("test-namespace"), "test-operator", fakeControllerRef(t))

	r.Event("TestReason", "foo")

	var createdEvent *corev1.Event

	for _, action := range client.Actions() {
		if action.Matches("create", "events") {
			createAction := action.(clientgotesting.CreateAction)
			createdEvent = createAction.GetObject().(*corev1.Event)
			break
		}
	}
	if createdEvent == nil {
		t.Fatalf("expected event to be created")
	}
	if createdEvent.InvolvedObject.Kind != "Deployment" {
		t.Errorf("expected involved object kind Deployment, got: %q", createdEvent.InvolvedObject.Kind)
	}
	if createdEvent.InvolvedObject.Namespace != "test-namespace" {
		t.Errorf("expected involved object namespace test-namespace, got: %q", createdEvent.InvolvedObject.Namespace)
	}
	if createdEvent.Reason != "TestReason" {
		t.Errorf("expected event to have TestReason, got %q", createdEvent.Reason)
	}
	if createdEvent.Message != "foo" {
		t.Errorf("expected message to be foo, got %q", createdEvent.Message)
	}
	if createdEvent.Type != "Normal" {
		t.Errorf("expected event type to be Normal, got %q", createdEvent.Type)
	}
	if createdEvent.Source.Component != "test-operator" {
		t.Errorf("expected event source to be test-operator, got %q", createdEvent.Source.Component)
	}
}

func TestGetControllerReferenceForCurrentPodIsPod(t *testing.T) {
	pod := fakePod("test", "test")
	pod.OwnerReferences = []metav1.OwnerReference{}
	client := fake.NewSimpleClientset(pod)

	podNameEnvFunc = func() string {
		return "test"
	}

	objectReference, err := GetControllerReferenceForCurrentPod(client, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if objectReference.Name != "test" {
		t.Errorf("expected objectReference name to be 'test', got %q", objectReference.Name)
	}

	if objectReference.GroupVersionKind().String() != "/v1, Kind=Pod" {
		t.Errorf("expected objectReference to be Pod, got %q", objectReference.GroupVersionKind().String())
	}
}

func TestGetControllerReferenceForCurrentPodIsReplicaSet(t *testing.T) {
	rs := fakeReplicaSet("test", "test")
	rs.OwnerReferences = []metav1.OwnerReference{}
	client := fake.NewSimpleClientset(fakePod("test", "test"), rs)

	podNameEnvFunc = func() string {
		return "test"
	}

	objectReference, err := GetControllerReferenceForCurrentPod(client, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if objectReference.Name != "test" {
		t.Errorf("expected objectReference name to be 'test', got %q", objectReference.Name)
	}

	if objectReference.GroupVersionKind().String() != "apps/v1, Kind=ReplicaSet" {
		t.Errorf("expected objectReference to be ReplicaSet, got %q", objectReference.GroupVersionKind().String())
	}
}

func TestGetControllerReferenceForCurrentPod(t *testing.T) {
	client := fake.NewSimpleClientset(fakePod("test", "test"), fakeReplicaSet("test", "test"))

	podNameEnvFunc = func() string {
		return "test"
	}

	objectReference, err := GetControllerReferenceForCurrentPod(client, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if objectReference.Name != "test" {
		t.Errorf("expected objectReference name to be 'test', got %q", objectReference.Name)
	}

	if objectReference.GroupVersionKind().String() != "apps/v1, Kind=Deployment" {
		t.Errorf("expected objectReference to be Deployment, got %q", objectReference.GroupVersionKind().String())
	}
}
