package installer

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
)

func TestNewNodeStateForInstallInProgress(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()

	var (
		installerPod   *v1.Pod
		createPodCount int
	)

	kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		installerPod = action.(ktesting.CreateAction).GetObject().(*v1.Pod)
		createPodCount += 1
		return false, nil, nil
	})

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))
	fakeStaticPodOperatorClient := common.NewFakeStaticPodOperatorClient(
		&operatorv1alpha1.OperatorSpec{
			ManagementState: operatorv1alpha1.Managed,
			Version:         "3.11.1",
		},
		&operatorv1alpha1.OperatorStatus{},
		&operatorv1alpha1.StaticPodOperatorStatus{
			LatestAvailableDeploymentGeneration: 1,
			NodeStatuses: []operatorv1alpha1.NodeStatus{
				{
					NodeName:                    "test-node-1",
					CurrentDeploymentGeneration: 0,
					TargetDeploymentGeneration:  0,
				},
			},
		},
		nil,
	)

	c := NewInstallerController(
		"test",
		[]string{"test-config"},
		[]string{"test-secret"},
		[]string{"/bin/true"},
		kubeInformers,
		fakeStaticPodOperatorClient,
		kubeClient,
	)
	c.installerPodImageFn = func() string { return "docker.io/foo/bar" }

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if installerPod == nil {
		t.Fatalf("expected to create installer pod")
	}

	_, currStatus, _, _ := fakeStaticPodOperatorClient.Get()
	currStatus.NodeStatuses[0].TargetDeploymentGeneration = 1
	fakeStaticPodOperatorClient.UpdateStatus("1", currStatus)

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if createPodCount != 1 {
		t.Fatalf("was not expecting to create new installer pod")
	}

	installerPod.Status.Phase = v1.PodSucceeded
	kubeClient.PrependReactor("get", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, installerPod, nil
	})

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.Get()
	if generation := currStatus.NodeStatuses[0].CurrentDeploymentGeneration; generation != 1 {
		t.Errorf("expected current deployment generation for node to be 1, got %d", generation)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.Get()
	currStatus.LatestAvailableDeploymentGeneration = 2
	currStatus.NodeStatuses[0].TargetDeploymentGeneration = 2
	currStatus.NodeStatuses[0].CurrentDeploymentGeneration = 1
	fakeStaticPodOperatorClient.UpdateStatus("1", currStatus)

	installerPod.Status.Phase = v1.PodFailed
	installerPod.Status.ContainerStatuses = []v1.ContainerStatus{
		{
			Name: "installer",
			State: v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{Message: "fake death"},
			},
		},
	}
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.Get()
	if generation := currStatus.NodeStatuses[0].LastFailedDeploymentGeneration; generation != 2 {
		t.Errorf("expected last failed deployment generation for node to be 2, got %d", generation)
	}

	if errors := currStatus.NodeStatuses[0].LastFailedDeploymentErrors; len(errors) > 0 {
		if errors[0] != "installer: fake death" {
			t.Errorf("expected the error to be set to 'fake death', got %#v", errors)
		}
	} else {
		t.Errorf("expected errors to be not empty")
	}
}

func TestCreateInstallerPod(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()

	var installerPod *v1.Pod
	kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		installerPod = action.(ktesting.CreateAction).GetObject().(*v1.Pod)
		return false, nil, nil
	})
	kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))

	fakeStaticPodOperatorClient := common.NewFakeStaticPodOperatorClient(
		&operatorv1alpha1.OperatorSpec{
			ManagementState: operatorv1alpha1.Managed,
			Version:         "3.11.1",
		},
		&operatorv1alpha1.OperatorStatus{},
		&operatorv1alpha1.StaticPodOperatorStatus{
			LatestAvailableDeploymentGeneration: 1,
			NodeStatuses: []operatorv1alpha1.NodeStatus{
				{
					NodeName:                    "test-node-1",
					CurrentDeploymentGeneration: 0,
					TargetDeploymentGeneration:  0,
				},
			},
		},
		nil,
	)

	c := NewInstallerController(
		"test",
		[]string{"test-config"},
		[]string{"test-secret"},
		[]string{"/bin/true"},
		kubeInformers,
		fakeStaticPodOperatorClient,
		kubeClient,
	)
	c.installerPodImageFn = func() string { return "docker.io/foo/bar" }
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if installerPod == nil {
		t.Fatalf("expected to create installer pod")
	}

	if installerPod.Spec.Containers[0].Image != "docker.io/foo/bar" {
		t.Fatalf("expected docker.io/foo/bar image, got %q", installerPod.Spec.Containers[0].Image)
	}

	if installerPod.Spec.Containers[0].Command[0] != "/bin/true" {
		t.Fatalf("expected /bin/true as a command, got %q", installerPod.Spec.Containers[0].Command[0])
	}

	if installerPod.Name != "installer-1-test-node-1" {
		t.Fatalf("expected name installer-1-test-node-1, got %q", installerPod.Name)
	}

	if installerPod.Namespace != "test" {
		t.Fatalf("expected test namespace, got %q", installerPod.Namespace)
	}

	expectedArgs := []string{
		"-v=0",
		"--deployment-id=1",
		"--namespace=test",
		"--pod=test-config",
		"--resource-dir=/etc/kubernetes/static-pod-resources",
		"--pod-manifest-dir=/etc/kubernetes/manifests",
		"--configmaps=test-config",
		"--secrets=test-secret",
	}

	if len(expectedArgs) != len(installerPod.Spec.Containers[0].Args) {
		t.Fatalf("expected arguments does not match container arguments: %#v != %#v", expectedArgs, installerPod.Spec.Containers[0].Args)
	}

	for i, v := range installerPod.Spec.Containers[0].Args {
		if expectedArgs[i] != v {
			t.Errorf("arg[%d] expected %q, got %q", i, expectedArgs[i], v)
		}
	}
}

func TestCreateInstallerPodMultiNode(t *testing.T) {
	tests := []struct {
		name                                string
		nodeStatuses                        []operatorv1alpha1.NodeStatus
		latestAvailableDeploymentGeneration int32
		evaluateInstallerPods               func(pods map[string]*v1.Pod) error
	}{
		{
			name: "three-nodes",
			latestAvailableDeploymentGeneration: 1,
			nodeStatuses: []operatorv1alpha1.NodeStatus{
				{
					NodeName: "test-node-1",
				},
				{
					NodeName: "test-node-2",
				},
				{
					NodeName: "test-node-3",
				},
			},
			evaluateInstallerPods: func(pods map[string]*v1.Pod) error {
				if len(pods) != 3 {
					return fmt.Errorf("expected 3 pods, got %d", len(pods))
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		installerPods := map[string]*v1.Pod{}

		kubeClient := fake.NewSimpleClientset()
		kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			createdPod := action.(ktesting.CreateAction).GetObject().(*v1.Pod)
			// Once the installer pod is created, set its status to succeeded.
			// Note that in reality, this will probably take couple sync cycles to happen, however it is useful to do this fast
			// to rule out timing bugs.
			createdPod.Status.Phase = v1.PodSucceeded
			installerPods[createdPod.Name] = createdPod
			return false, nil, nil
		})

		// When newNodeStateForInstallInProgress ask for pod, give it a pod that already succeeded.
		kubeClient.PrependReactor("get", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			podName := action.(ktesting.GetAction).GetName()
			pod, exists := installerPods[podName]
			if !exists {
				return false, nil, nil
			}
			return true, pod, nil
		})

		kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test-"+test.name))
		fakeStaticPodOperatorClient := common.NewFakeStaticPodOperatorClient(
			&operatorv1alpha1.OperatorSpec{
				ManagementState: operatorv1alpha1.Managed,
				Version:         "3.11.1",
			},
			&operatorv1alpha1.OperatorStatus{},
			&operatorv1alpha1.StaticPodOperatorStatus{
				LatestAvailableDeploymentGeneration: test.latestAvailableDeploymentGeneration,
				NodeStatuses:                        test.nodeStatuses,
			},
			nil,
		)

		c := NewInstallerController(
			"test-"+test.name,
			[]string{"test-config"},
			[]string{"test-secret"},
			[]string{"/bin/true"},
			kubeInformers,
			fakeStaticPodOperatorClient,
			kubeClient,
		)
		c.installerPodImageFn = func() string { return "docker.io/foo/bar" }

		// Each node need at least 2 syncs to first create the pod and then acknowledge its existence...
		for i := 1; i <= len(test.nodeStatuses)*2; i++ {
			if err := c.sync(); err != nil {
				t.Errorf("%s: failed to execute %d sync: %v", test.name, i, err)
			}
		}

		if err := test.evaluateInstallerPods(installerPods); err != nil {
			t.Errorf("%s: installer pods failed evaluation: %v", test.name, err)
		}
	}

}
