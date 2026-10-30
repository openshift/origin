package authentication

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestDeploymentRolledOut(t *testing.T) {
	t.Parallel()
	const previousGeneration = 4

	rolledOut := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Generation: previousGeneration + 1},
			Spec:       appsv1.DeploymentSpec{Replicas: ptr.To[int32](3)},
			Status: appsv1.DeploymentStatus{
				ObservedGeneration: previousGeneration + 1,
				Replicas:           3,
				UpdatedReplicas:    3,
				ReadyReplicas:      3,
				AvailableReplicas:  3,
			},
		}
	}

	for _, testCase := range []struct {
		name   string
		mutate func(*appsv1.Deployment)
		want   bool
	}{
		{name: "complete rollout of a new generation", want: true},
		{name: "generation unchanged", mutate: func(d *appsv1.Deployment) {
			d.Generation, d.Status.ObservedGeneration = previousGeneration, previousGeneration
		}},
		{name: "new generation not yet observed", mutate: func(d *appsv1.Deployment) {
			d.Status.ObservedGeneration = previousGeneration
		}},
		{name: "replicas of the previous generation remaining", mutate: func(d *appsv1.Deployment) {
			d.Status.Replicas = 4
		}},
		{name: "not all replicas updated", mutate: func(d *appsv1.Deployment) {
			d.Status.UpdatedReplicas = 2
		}},
		{name: "not all replicas ready", mutate: func(d *appsv1.Deployment) {
			d.Status.ReadyReplicas = 2
		}},
		{name: "not all replicas available", mutate: func(d *appsv1.Deployment) {
			d.Status.AvailableReplicas = 2
		}},
		{name: "unavailable replicas", mutate: func(d *appsv1.Deployment) {
			d.Status.UnavailableReplicas = 1
		}},
		{name: "scaled to zero", mutate: func(d *appsv1.Deployment) {
			d.Spec.Replicas = ptr.To[int32](0)
			d.Status.Replicas, d.Status.UpdatedReplicas, d.Status.ReadyReplicas, d.Status.AvailableReplicas = 0, 0, 0, 0
		}},
		{name: "nil desired replicas defaults to one", want: true, mutate: func(d *appsv1.Deployment) {
			d.Spec.Replicas = nil
			d.Status.Replicas, d.Status.UpdatedReplicas, d.Status.ReadyReplicas, d.Status.AvailableReplicas = 1, 1, 1, 1
		}},
		{name: "nil desired replicas with more than one replica", mutate: func(d *appsv1.Deployment) {
			d.Spec.Replicas = nil
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			deployment := rolledOut()
			if testCase.mutate != nil {
				testCase.mutate(deployment)
			}
			got, description := deploymentRolledOut(deployment, previousGeneration)
			if got != testCase.want {
				t.Errorf("deploymentRolledOut() = %v, want %v (%s)", got, testCase.want, description)
			}
		})
	}
}
