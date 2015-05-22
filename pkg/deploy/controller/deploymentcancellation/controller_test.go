package deploymentcancellation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_cancelInflightDeployment ensures that a deployment in the
// new/pending/running state can be cancelled
// and a deployment in failed/complete state cannot be cancelled
func TestHandle_cancelInflightDeployment(t *testing.T) {
	var updatedPod *kapi.Pod
	deployment := cancelledDeployment()

	controller := &DeploymentCancellationController{
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return okPod(), nil
			},
			updatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				updatedPod = pod
				return updatedPod, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	// test inflight statuses
	inflightStatus := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
	}
	for _, status := range inflightStatus {
		updatedPod = nil
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)

		err := controller.Handle(deployment)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedPod == nil {
			t.Fatalf("expected an updated deployer pod")
		}

		if *updatedPod.Spec.ActiveDeadlineSeconds != int64(0) {
			t.Fatalf("ActiveDeadlineSeconds not set to 0")
		}
	}

	// test terminal statuses
	terminalStatus := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusComplete,
		deployapi.DeploymentStatusFailed,
	}
	for _, status := range terminalStatus {
		updatedPod = nil
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)
		err := controller.Handle(deployment)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedPod != nil {
			t.Fatalf("unexpected deployer pod update")
		}
	}
}

// TestHandle_podActiveDeadlineNonZero ensures that a deployer pod
// that can be cancelled and has an ActiveDeadlineSeconds value
// of non-zero is updated correctly
func TestHandle_podActiveDeadlineNonZero(t *testing.T) {
	var updatedPod *kapi.Pod
	deployment := cancelledDeployment()

	controller := &DeploymentCancellationController{
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return ttlNonZeroPod(), nil
			},
			updatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				updatedPod = pod
				return updatedPod, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	err := controller.Handle(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedPod == nil {
		t.Fatalf("expected an updated deployer pod")
	}

	if *updatedPod.Spec.ActiveDeadlineSeconds != int64(0) {
		t.Fatalf("ActiveDeadlineSeconds not set to 0")
	}
}

// TestHandle_podActiveDeadlineZero ensures that a deployer pod
// that can be cancelled and has an ActiveDeadlineSeconds value
// already set to zero is not updated again
func TestHandle_podActiveDeadlineZero(t *testing.T) {
	controller := &DeploymentCancellationController{
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return ttlZeroPod(), nil
			},
			updatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				t.Fatalf("unexpected pod update")
				return nil, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	deployment := cancelledDeployment()
	err := controller.Handle(deployment)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func cancelledDeployment() *kapi.ReplicationController {
	config := deploytest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
	return deployment
}

func okPod() *kapi.Pod {
	return &kapi.Pod{}
}

func ttlNonZeroPod() *kapi.Pod {
	ttl := int64(10)
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			ActiveDeadlineSeconds: &ttl,
		},
	}
}

func ttlZeroPod() *kapi.Pod {
	ttl := int64(0)
	return &kapi.Pod{
		Spec: kapi.PodSpec{
			ActiveDeadlineSeconds: &ttl,
		},
	}
}
