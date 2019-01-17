package installer

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestNewNodeStateForInstallInProgress(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()

	var installerPod *v1.Pod

	kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		if installerPod != nil {
			return true, nil, errors.NewAlreadyExists(schema.GroupResource{Resource: "pods"}, installerPod.Name)
		}
		installerPod = action.(ktesting.CreateAction).GetObject().(*v1.Pod)
		kubeClient.PrependReactor("get", "pods", getPodsReactor(installerPod))
		return true, installerPod, nil
	})

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))
	fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
		&operatorv1.OperatorSpec{
			ManagementState: operatorv1.Managed,
		},
		&operatorv1.OperatorStatus{},
		&operatorv1.StaticPodOperatorStatus{
			LatestAvailableRevision: 1,
			NodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-1",
					CurrentRevision: 0,
					TargetRevision:  0,
				},
			},
		},
		nil,
	)

	eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})
	podCommand := []string{"/bin/true", "--foo=test", "--bar"}
	c := NewInstallerController(
		"test", "test-pod",
		[]string{"test-config"},
		[]string{"test-secret"},
		podCommand,
		kubeInformers,
		fakeStaticPodOperatorClient,
		kubeClient,
		eventRecorder,
	)
	c.installerPodImageFn = func() string { return "docker.io/foo/bar" }

	t.Log("setting target revision")
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if installerPod != nil {
		t.Fatalf("not expected to create installer pod yet")
	}

	_, currStatus, _, _ := fakeStaticPodOperatorClient.GetStaticPodOperatorState()
	if currStatus.NodeStatuses[0].TargetRevision != 1 {
		t.Fatalf("expected target revision generation 1, got: %d", currStatus.NodeStatuses[0].TargetRevision)
	}

	t.Log("starting installer pod")

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}
	if installerPod == nil {
		t.Fatalf("expected to create installer pod")
	}

	t.Run("VerifyPodCommand", func(t *testing.T) {
		cmd := installerPod.Spec.Containers[0].Command
		if !reflect.DeepEqual(podCommand, cmd) {
			t.Errorf("expected pod command %#v to match resulting installer pod command: %#v", podCommand, cmd)
		}
	})

	t.Run("VerifyPodArguments", func(t *testing.T) {
		args := installerPod.Spec.Containers[0].Args
		if len(args) == 0 {
			t.Errorf("pod args should not be empty")
		}
		foundRevision := false
		for _, arg := range args {
			if arg == "--revision=1" {
				foundRevision = true
			}
		}
		if !foundRevision {
			t.Errorf("revision installer argument not found")
		}
	})

	t.Log("synching again, nothing happens")
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if currStatus.NodeStatuses[0].TargetRevision != 1 {
		t.Fatalf("expected target revision generation 1, got: %d", currStatus.NodeStatuses[0].TargetRevision)
	}
	if currStatus.NodeStatuses[0].CurrentRevision != 0 {
		t.Fatalf("expected current revision generation 0, got: %d", currStatus.NodeStatuses[0].CurrentRevision)
	}

	t.Log("installer succeeded")
	installerPod.Status.Phase = v1.PodSucceeded

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.GetStaticPodOperatorState()
	if generation := currStatus.NodeStatuses[0].CurrentRevision; generation != 0 {
		t.Errorf("expected current revision generation for node to be 0, got %d", generation)
	}

	t.Log("static pod launched, but is not ready")
	staticPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-test-node-1",
			Namespace: "test",
			Labels:    map[string]string{"revision": "1"},
		},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Conditions: []v1.PodCondition{
				{
					Status: v1.ConditionFalse,
					Type:   v1.PodReady,
				},
			},
			Phase: v1.PodRunning,
		},
	}
	kubeClient.PrependReactor("get", "pods", getPodsReactor(staticPod))

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.GetStaticPodOperatorState()
	if generation := currStatus.NodeStatuses[0].CurrentRevision; generation != 0 {
		t.Errorf("expected current revision generation for node to be 0, got %d", generation)
	}

	t.Log("static pod is ready")
	staticPod.Status.Conditions[0].Status = v1.ConditionTrue

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.GetStaticPodOperatorState()
	if generation := currStatus.NodeStatuses[0].CurrentRevision; generation != 1 {
		t.Errorf("expected current revision generation for node to be 1, got %d", generation)
	}

	_, currStatus, _, _ = fakeStaticPodOperatorClient.GetStaticPodOperatorState()
	currStatus.LatestAvailableRevision = 2
	currStatus.NodeStatuses[0].TargetRevision = 2
	currStatus.NodeStatuses[0].CurrentRevision = 1
	fakeStaticPodOperatorClient.UpdateStaticPodOperatorStatus("1", currStatus)

	installerPod.Name = "installer-2-test-node-1"
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

	_, currStatus, _, _ = fakeStaticPodOperatorClient.GetStaticPodOperatorState()
	if generation := currStatus.NodeStatuses[0].LastFailedRevision; generation != 2 {
		t.Errorf("expected last failed revision generation for node to be 2, got %d", generation)
	}

	if errors := currStatus.NodeStatuses[0].LastFailedRevisionErrors; len(errors) > 0 {
		if errors[0] != "installer: fake death" {
			t.Errorf("expected the error to be set to 'fake death', got %#v", errors)
		}
	} else {
		t.Errorf("expected errors to be not empty")
	}

	if v1helpers.FindOperatorCondition(currStatus.Conditions, operatorv1.OperatorStatusTypeProgressing) == nil {
		t.Error("missing Progressing")
	}
	if v1helpers.FindOperatorCondition(currStatus.Conditions, operatorv1.OperatorStatusTypeAvailable) == nil {
		t.Error("missing Available")
	}
}

func getPodsReactor(pods ...*v1.Pod) ktesting.ReactionFunc {
	return func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		podName := action.(ktesting.GetAction).GetName()
		for _, p := range pods {
			if p.Namespace == action.GetNamespace() && p.Name == podName {
				return true, p, nil
			}
		}
		return false, nil, nil
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

	fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
		&operatorv1.OperatorSpec{
			ManagementState: operatorv1.Managed,
		},
		&operatorv1.OperatorStatus{},
		&operatorv1.StaticPodOperatorStatus{
			LatestAvailableRevision: 1,
			NodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-1",
					CurrentRevision: 0,
					TargetRevision:  0,
				},
			},
		},
		nil,
	)
	eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})

	c := NewInstallerController(
		"test", "test-pod",
		[]string{"test-config"},
		[]string{"test-secret"},
		[]string{"/bin/true"},
		kubeInformers,
		fakeStaticPodOperatorClient,
		kubeClient,
		eventRecorder,
	)
	c.installerPodImageFn = func() string { return "docker.io/foo/bar" }
	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if installerPod != nil {
		t.Fatalf("expected first sync not to create installer pod")
	}

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
		"-v=4",
		"--revision=1",
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
	newStaticPod := func(name string, id int, phase v1.PodPhase, ready bool) *v1.Pod {
		condStatus := v1.ConditionTrue
		if !ready {
			condStatus = v1.ConditionFalse
		}
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "test",
				Labels:    map[string]string{"revision": strconv.Itoa(id)},
			},
			Spec: v1.PodSpec{},
			Status: v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Status: condStatus,
						Type:   v1.PodReady,
					},
				},
				Phase: phase,
			},
		}
	}

	tests := []struct {
		name                    string
		nodeStatuses            []operatorv1.NodeStatus
		staticPods              []*v1.Pod
		latestAvailableRevision int32
		expectedUpgradeOrder    []int
		expectedSyncError       []bool
		updateStatusErrors      []error
	}{
		{
			name: "three fresh nodes",
			latestAvailableRevision: 1,
			nodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName: "test-node-0",
				},
				{
					NodeName: "test-node-1",
				},
				{
					NodeName: "test-node-2",
				},
			},
			expectedUpgradeOrder: []int{0, 1, 2},
		},
		{
			name: "three nodes with current revision, all static pods ready",
			latestAvailableRevision: 2,
			nodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-0",
					CurrentRevision: 1,
				},
				{
					NodeName:        "test-node-1",
					CurrentRevision: 1,
				},
				{
					NodeName:        "test-node-2",
					CurrentRevision: 1,
				},
			},
			staticPods: []*v1.Pod{
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-1"), 1, v1.PodRunning, true),
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-2"), 1, v1.PodRunning, true),
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-3"), 1, v1.PodRunning, true),
			},
			expectedUpgradeOrder: []int{0, 1, 2},
		},
		{
			name: "three nodes with current revision, second static pods unread",
			latestAvailableRevision: 2,
			nodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-1",
					CurrentRevision: 1,
				},
				{
					NodeName:        "test-node-2",
					CurrentRevision: 1,
				},
				{
					NodeName:        "test-node-3",
					CurrentRevision: 1,
				},
			},
			staticPods: []*v1.Pod{
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-1"), 1, v1.PodRunning, true),
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-2"), 1, v1.PodRunning, false),
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-3"), 1, v1.PodRunning, true),
			},
			expectedUpgradeOrder: []int{1, 0, 2},
		},
		{
			name: "three nodes with current revision, 2nd & 3rd static pods unread",
			latestAvailableRevision: 2,
			nodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-1",
					CurrentRevision: 1,
				},
				{
					NodeName:        "test-node-2",
					CurrentRevision: 1,
				},
				{
					NodeName:        "test-node-3",
					CurrentRevision: 1,
				},
			},
			staticPods: []*v1.Pod{
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-1"), 1, v1.PodRunning, true),
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-2"), 1, v1.PodRunning, false),
				newStaticPod(mirrorPodNameForNode("test-pod", "test-node-3"), 1, v1.PodRunning, false),
			},
			expectedUpgradeOrder: []int{1, 2, 0},
		},
		{
			name: "first update status fails",
			latestAvailableRevision: 2,
			nodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName: "test-node-1",
				},
			},
			expectedUpgradeOrder: []int{0},
			updateStatusErrors:   []error{errors.NewInternalError(fmt.Errorf("unknown"))},
			expectedSyncError:    []bool{true},
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createdInstallerPods := []*v1.Pod{}
			updatedStaticPods := map[string]*v1.Pod{}

			installerNodeAndID := func(installerName string) (string, int) {
				ss := strings.SplitN(strings.TrimPrefix(installerName, "installer-"), "-", 2)
				id, err := strconv.Atoi(ss[0])
				if err != nil {
					t.Fatalf("unexpected id derived from install pod name %q: %v", installerName, err)
				}
				return ss[1], id
			}

			kubeClient := fake.NewSimpleClientset()
			kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				createdPod := action.(ktesting.CreateAction).GetObject().(*v1.Pod)
				// Once the installer pod is created, set its status to succeeded.
				// Note that in reality, this will probably take couple sync cycles to happen, however it is useful to do this fast
				// to rule out timing bugs.
				createdPod.Status.Phase = v1.PodSucceeded
				createdInstallerPods = append(createdInstallerPods, createdPod)

				nodeName, id := installerNodeAndID(createdPod.Name)
				staticPodName := mirrorPodNameForNode("test-pod", nodeName)

				updatedStaticPods[staticPodName] = newStaticPod(staticPodName, id, v1.PodRunning, true)

				return false, nil, nil
			})

			// When newNodeStateForInstallInProgress ask for pod, give it a pod that already succeeded.
			kubeClient.PrependReactor("get", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				podName := action.(ktesting.GetAction).GetName()
				for i := len(createdInstallerPods) - 1; i >= 0; i-- {
					pod := createdInstallerPods[i]
					if pod.Name == podName {
						return true, pod, nil
					}
				}
				if pod, exists := updatedStaticPods[podName]; exists {
					if pod == nil {
						return false, nil, nil
					}
					return true, pod, nil
				}
				for _, pod := range test.staticPods {
					if pod.Name == podName {
						return true, pod, nil
					}
				}
				return false, nil, nil
			})

			kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test-"+test.name))
			statusUpdateCount := 0
			statusUpdateErrorFunc := func(rv string, status *operatorv1.StaticPodOperatorStatus) error {
				var err error
				if statusUpdateCount < len(test.updateStatusErrors) {
					err = test.updateStatusErrors[statusUpdateCount]
				}
				statusUpdateCount++
				return err
			}
			fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: test.latestAvailableRevision,
					NodeStatuses:            test.nodeStatuses,
				},
				statusUpdateErrorFunc,
			)

			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})

			c := NewInstallerController(
				fmt.Sprintf("test-%d", i), "test-pod",
				[]string{"test-config"},
				[]string{"test-secret"},
				[]string{"/bin/true"},
				kubeInformers,
				fakeStaticPodOperatorClient,
				kubeClient,
				eventRecorder,
			)
			c.installerPodImageFn = func() string { return "docker.io/foo/bar" }

			// Each node need at least 2 syncs to first create the pod and then acknowledge its existence.
			for i := 1; i <= len(test.nodeStatuses)*2+1; i++ {
				err := c.sync()
				expectedErr := false
				if i-1 < len(test.expectedSyncError) && test.expectedSyncError[i-1] {
					expectedErr = true
				}
				if err != nil && !expectedErr {
					t.Errorf("failed to execute %d sync: %v", i, err)
				} else if err == nil && expectedErr {
					t.Errorf("expected sync error in sync %d, but got nil", i)
				}
			}

			for i := range test.expectedUpgradeOrder {
				if i >= len(createdInstallerPods) {
					t.Fatalf("expected more installer pod in the node order %v", test.expectedUpgradeOrder[i:])
				}

				nodeName, _ := installerNodeAndID(createdInstallerPods[i].Name)
				if expected, got := test.nodeStatuses[test.expectedUpgradeOrder[i]].NodeName, nodeName; expected != got {
					t.Errorf("expected installer pod number %d to be for node %q, but got %q", i, expected, got)
				}
			}
			if len(test.expectedUpgradeOrder) < len(createdInstallerPods) {
				t.Errorf("too many installer pods created: %#v", createdInstallerPods[len(test.expectedUpgradeOrder):])
			}
		})
	}

}

func TestInstallerController_manageInstallationPods(t *testing.T) {
	type fields struct {
		targetNamespace      string
		staticPodName        string
		configMaps           []string
		secrets              []string
		command              []string
		operatorConfigClient v1helpers.StaticPodOperatorClient
		kubeClient           kubernetes.Interface
		eventRecorder        events.Recorder
		queue                workqueue.RateLimitingInterface
		installerPodImageFn  func() string
	}
	type args struct {
		operatorSpec           *operatorv1.OperatorSpec
		originalOperatorStatus *operatorv1.StaticPodOperatorStatus
		resourceVersion        string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &InstallerController{
				targetNamespace:      tt.fields.targetNamespace,
				staticPodName:        tt.fields.staticPodName,
				configMaps:           tt.fields.configMaps,
				secrets:              tt.fields.secrets,
				command:              tt.fields.command,
				operatorConfigClient: tt.fields.operatorConfigClient,
				kubeClient:           tt.fields.kubeClient,
				eventRecorder:        tt.fields.eventRecorder,
				queue:                tt.fields.queue,
				installerPodImageFn:  tt.fields.installerPodImageFn,
			}
			got, err := c.manageInstallationPods(tt.args.operatorSpec, tt.args.originalOperatorStatus, tt.args.resourceVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("InstallerController.manageInstallationPods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("InstallerController.manageInstallationPods() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeToStartRevisionWith(t *testing.T) {
	type StaticPod struct {
		name     string
		state    staticPodState
		revision int32
	}
	type Test struct {
		name        string
		nodes       []operatorv1.NodeStatus
		pods        []StaticPod
		expected    int
		expectedErr bool
	}

	newNode := func(name string, current, target int32) operatorv1.NodeStatus {
		return operatorv1.NodeStatus{NodeName: name, CurrentRevision: current, TargetRevision: target}
	}

	for _, test := range []Test{
		{
			name:        "empty",
			expectedErr: true,
		},
		{
			name: "no pods",
			pods: nil,
			nodes: []operatorv1.NodeStatus{
				newNode("a", 0, 0),
				newNode("b", 0, 0),
				newNode("c", 0, 0),
			},
			expected: 0,
		},
		{
			name: "all ready",
			pods: []StaticPod{
				{"a", staticPodStateReady, 1},
				{"b", staticPodStateReady, 1},
				{"c", staticPodStateReady, 1},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 1, 0),
				newNode("b", 1, 0),
				newNode("c", 1, 0),
			},
			expected: 0,
		},
		{
			name: "one failed",
			pods: []StaticPod{
				{"a", staticPodStateReady, 1},
				{"b", staticPodStateReady, 1},
				{"c", staticPodStateFailed, 1},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 1, 0),
				newNode("b", 1, 0),
				newNode("c", 1, 0),
			},
			expected: 2,
		},
		{
			name: "one pending",
			pods: []StaticPod{
				{"a", staticPodStateReady, 1},
				{"b", staticPodStateReady, 1},
				{"c", staticPodStatePending, 1},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 1, 0),
				newNode("b", 1, 0),
				newNode("c", 0, 0),
			},
			expected: 2,
		},
		{
			name: "multiple pending",
			pods: []StaticPod{
				{"a", staticPodStateReady, 1},
				{"b", staticPodStatePending, 1},
				{"c", staticPodStatePending, 1},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 1, 0),
				newNode("b", 0, 0),
				newNode("c", 0, 0),
			},
			expected: 1,
		},
		{
			name: "one updating",
			pods: []StaticPod{
				{"a", staticPodStateReady, 1},
				{"b", staticPodStatePending, 0},
				{"c", staticPodStateReady, 0},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 1, 0),
				newNode("b", 0, 1),
				newNode("c", 0, 0),
			},
			expected: 1,
		},
		{
			name: "pods missing",
			pods: []StaticPod{
				{"a", staticPodStateReady, 1},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 1, 0),
				newNode("b", 0, 0),
				newNode("c", 0, 0),
			},
			expected: 1,
		},
		{
			name: "one old",
			pods: []StaticPod{
				{"a", staticPodStateReady, 2},
				{"b", staticPodStateReady, 1},
				{"c", staticPodStateReady, 2},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 2, 0),
				newNode("b", 2, 0),
				newNode("c", 2, 0),
			},
			expected: 1,
		},
		{
			name: "one behind, but as stated",
			pods: []StaticPod{
				{"a", staticPodStateReady, 2},
				{"b", staticPodStateReady, 1},
				{"c", staticPodStateReady, 2},
			},
			nodes: []operatorv1.NodeStatus{
				newNode("a", 2, 0),
				newNode("b", 1, 0),
				newNode("c", 2, 0),
			},
			expected: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fakeGetStaticPodState := func(nodeName string) (state staticPodState, revision string, errs []string, err error) {
				for _, p := range test.pods {
					if p.name == nodeName {
						return p.state, strconv.Itoa(int(p.revision)), nil, nil
					}
				}
				return staticPodStatePending, "", nil, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, nodeName)
			}
			i, err := nodeToStartRevisionWith(fakeGetStaticPodState, test.nodes)
			if err == nil && test.expectedErr {
				t.Fatalf("expected error, got none")
			}
			if err != nil && !test.expectedErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if i != test.expected {
				t.Errorf("expected node ID %d, got %d", test.expected, i)
			}
		})
	}
}

func TestSetConditions(t *testing.T) {

	type TestCase struct {
		name                    string
		latestAvailableRevision int32
		currentRevisions        []int32
		expectedAvailableStatus operatorv1.ConditionStatus
		expectedPendingStatus   operatorv1.ConditionStatus
	}

	testCase := func(name string, available, pending bool, latest int32, current ...int32) TestCase {
		availableStatus := operatorv1.ConditionFalse
		pendingStatus := operatorv1.ConditionFalse
		if available {
			availableStatus = operatorv1.ConditionTrue
		}
		if pending {
			pendingStatus = operatorv1.ConditionTrue
		}
		return TestCase{name, latest, current, availableStatus, pendingStatus}
	}

	testCases := []TestCase{
		testCase("AvailablePending", true, true, 2, 2, 1, 2, 1),
		testCase("AvailableNotPending", true, false, 2, 2, 2, 2),
		testCase("NotAvailablePending", false, true, 2, 1, 1),
		testCase("NotAvailableNotPending", false, false, 2),
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := &operatorv1.StaticPodOperatorStatus{
				LatestAvailableRevision: tc.latestAvailableRevision,
			}
			for _, current := range tc.currentRevisions {
				status.NodeStatuses = append(status.NodeStatuses, operatorv1.NodeStatus{CurrentRevision: current})
			}
			setAvailableProgressingConditions(status)
			availableCondition := v1helpers.FindOperatorCondition(status.Conditions, operatorv1.OperatorStatusTypeAvailable)
			if availableCondition == nil {
				t.Error("Available condition: not found")
			} else if availableCondition.Status != tc.expectedAvailableStatus {
				t.Errorf("Available condition: expected status %v, actual status %v", tc.expectedAvailableStatus, availableCondition.Status)
			}
			pendingCondition := v1helpers.FindOperatorCondition(status.Conditions, operatorv1.OperatorStatusTypeProgressing)
			if pendingCondition == nil {
				t.Error("Pending condition: not found")
			} else if pendingCondition.Status != tc.expectedPendingStatus {
				t.Errorf("Pending condition: expected status %v, actual status %v", tc.expectedPendingStatus, pendingCondition.Status)
			}
		})
	}

}
