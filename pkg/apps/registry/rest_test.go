package registry

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func TestWaitForRunningDeploymentSuccess(t *testing.T) {
	fakeController := &kapi.ReplicationController{}
	fakeController.Name = "test-1"
	fakeController.Namespace = "test"
	fakeController.Annotations = map[string]string{appsapi.DeploymentStatusAnnotation: string(appsapi.DeploymentStatusRunning)}

	kubeclient := fake.NewSimpleClientset([]runtime.Object{fakeController}...)
	fakeWatch := watch.NewFake()
	kubeclient.PrependWatchReactor("replicationcontrollers", clientgotesting.DefaultWatchReactor(fakeWatch, nil))
	stopChan := make(chan struct{})

	go func() {
		defer close(stopChan)
		rc, ok, err := WaitForRunningDeployment(kubeclient.Core(), fakeController, 10*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Errorf("expected to return success")
		}
		if rc == nil {
			t.Errorf("expected returned replication controller to not be nil")
		}
	}()

	fakeWatch.Modify(fakeController)
	<-stopChan
}

func TestWaitForRunningDeploymentRestartWatch(t *testing.T) {
	fakeController := &kapi.ReplicationController{}
	fakeController.Name = "test-1"
	fakeController.Namespace = "test"

	kubeclient := fake.NewSimpleClientset([]runtime.Object{fakeController}...)
	fakeWatch := watch.NewFake()

	watchCalledChan := make(chan struct{})
	kubeclient.PrependWatchReactor("replicationcontrollers", func(action clientgotesting.Action) (bool, watch.Interface, error) {
		fakeWatch.Reset()
		watchCalledChan <- struct{}{}
		return clientgotesting.DefaultWatchReactor(fakeWatch, nil)(action)
	})

	getReceivedChan := make(chan struct{})
	kubeclient.PrependReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		close(getReceivedChan)
		return true, fakeController, nil
	})

	stopChan := make(chan struct{})
	go func() {
		defer close(stopChan)
		rc, ok, err := WaitForRunningDeployment(kubeclient.Core(), fakeController, 10*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Errorf("expected to return success")
		}
		if rc == nil {
			t.Errorf("expected returned replication controller to not be nil")
		}
	}()

	select {
	case <-watchCalledChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for the watch to start")
	}

	// Send the StatusReasonGone error to watcher which should trigger the watch restart.
	goneError := &metav1.Status{Reason: metav1.StatusReasonGone}
	fakeWatch.Error(goneError)

	// Make sure we observed the "get" action on replication controller, so the watch gets
	// the latest resourceVersion.
	select {
	case <-getReceivedChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for get on replication controllers")
	}

	// Wait for the watcher to restart and then transition the replication controller to
	// running state.
	select {
	case <-watchCalledChan:
		fakeController.Annotations = map[string]string{appsapi.DeploymentStatusAnnotation: string(appsapi.DeploymentStatusRunning)}
		fakeWatch.Modify(fakeController)
		<-stopChan
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for the watch restart")
	}
}
