package deploylog

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	appsv1 "github.com/openshift/api/apps/v1"
)

func TestWaitForRunningDeploymentSuccess(t *testing.T) {
	fakeController := &corev1.ReplicationController{}
	fakeController.Name = "test-1"
	fakeController.Namespace = "test"
	fakeController.Annotations = map[string]string{appsv1.DeploymentStatusAnnotation: string(appsv1.DeploymentStatusRunning)}

	kubeclient := fake.NewSimpleClientset([]runtime.Object{fakeController}...)
	fakeWatch := watch.NewFake()
	kubeclient.PrependWatchReactor("replicationcontrollers", clientgotesting.DefaultWatchReactor(fakeWatch, nil))
	stopChan := make(chan struct{})

	go func() {
		defer close(stopChan)
		rc, err := WaitForRunningDeployment(kubeclient.CoreV1(), fakeController, 10*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rc == nil {
			t.Errorf("expected returned replication controller to not be nil")
		}
	}()

	fakeWatch.Modify(fakeController)
	<-stopChan
}
