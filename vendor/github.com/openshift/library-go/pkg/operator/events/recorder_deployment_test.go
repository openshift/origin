package events

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeFakeReplicaSet(namespace, name string) *appsv1.ReplicaSet {
	rs := appsv1.ReplicaSet{}
	rs.Name = name
	rs.Namespace = namespace
	rs.TypeMeta.Kind = "ReplicaSet"
	rs.TypeMeta.APIVersion = "apps/v1"
	return &rs
}

func TestGetReplicaSetOwnerReference(t *testing.T) {
	client := fake.NewSimpleClientset(makeFakeReplicaSet("test", "foo"))

	eventSourceReplicaSetNameEnvFunc = func() string {
		return "foo"
	}

	objectReference, err := GetReplicaSetOwnerReference(client.AppsV1().ReplicaSets("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if objectReference.Name != "foo" {
		t.Errorf("expected objectReference name to be 'foo', got %q", objectReference.Name)
	}

	if objectReference.GroupVersionKind().String() != "apps/v1, Kind=ReplicaSet" {
		t.Errorf("expected objectReference to be ReplicaSet, got %q", objectReference.GroupVersionKind().String())
	}
}
