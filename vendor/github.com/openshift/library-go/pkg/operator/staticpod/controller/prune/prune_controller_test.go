package prune

import (
	"fmt"
	"testing"

	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
)

type configMapInfo struct {
	name      string
	namespace string
	revision  string
	phase     string
}

func TestPruneAPIResources(t *testing.T) {
	tests := []struct {
		name            string
		targetNamespace string
		failedLimit     int32
		succeededLimit  int32
		currentRevision int
		configMaps      []configMapInfo
		testSecrets     []string
		testConfigs     []string
		startingObjects []runtime.Object
		expectedObjects []runtime.Object
	}{
		{
			name:            "prunes api resources based on limits set and status stored in configmap",
			targetNamespace: "prune-api",
			startingObjects: []runtime.Object{
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "1",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-2", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "2",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-3", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodFailed),
						"revision": "3",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-4", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodFailed),
						"revision": "4",
					},
				},
			},
			failedLimit:    1,
			succeededLimit: 1,
			expectedObjects: []runtime.Object{
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-2", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "2",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-4", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodFailed),
						"revision": "4",
					},
				},
			},
		},
		{
			name:            "protects InProgress and unknown revision statuses",
			targetNamespace: "prune-api",
			startingObjects: []runtime.Object{
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "1",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-2", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   "foo",
						"revision": "2",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-3", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   "InProgress",
						"revision": "3",
					},
				},
			},
			failedLimit:    1,
			succeededLimit: 1,
			expectedObjects: []runtime.Object{
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "1",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-2", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   "foo",
						"revision": "2",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-3", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   "InProgress",
						"revision": "3",
					},
				},
			},
		},
		{
			name:            "protects all with unlimited revisions",
			targetNamespace: "prune-api",
			startingObjects: []runtime.Object{
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "1",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-2", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "2",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-3", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "3",
					},
				},
			},
			failedLimit:    -1,
			succeededLimit: -1,
			expectedObjects: []runtime.Object{
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-1", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "1",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-2", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "2",
					},
				},
				&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "revision-status-3", Namespace: "prune-api"},
					Data: map[string]string{
						"status":   string(v1.PodSucceeded),
						"revision": "3",
					},
				},
			},
		},
	}
	for _, tc := range tests {
		kubeClient := fake.NewSimpleClientset(tc.startingObjects...)
		fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
			&operatorv1.OperatorSpec{
				ManagementState: operatorv1.Managed,
			},
			&operatorv1.OperatorStatus{},
			&operatorv1.StaticPodOperatorSpec{
				FailedRevisionLimit:    tc.failedLimit,
				SucceededRevisionLimit: tc.succeededLimit,
				OperatorSpec: operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
			},
			&operatorv1.StaticPodOperatorStatus{
				LatestAvailableRevision: 1,
				NodeStatuses: []operatorv1.NodeStatus{
					{
						NodeName:        "test-node-1",
						CurrentRevision: 1,
						TargetRevision:  0,
					},
				},
			},
			nil,
		)
		eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})

		operatorStatus := &operatorv1.StaticPodOperatorStatus{
			LatestAvailableRevision: 1,
			NodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-1",
					CurrentRevision: 1,
					TargetRevision:  0,
				},
			},
		}

		c := &PruneController{
			targetNamespace:      tc.targetNamespace,
			podResourcePrefix:    "test-pod",
			command:              []string{"/bin/true"},
			configMapGetter:      kubeClient.CoreV1(),
			secretGetter:         kubeClient.CoreV1(),
			podGetter:            kubeClient.CoreV1(),
			eventRecorder:        eventRecorder,
			operatorConfigClient: fakeStaticPodOperatorClient,
		}
		c.ownerRefsFn = func(revision int32) ([]metav1.OwnerReference, error) {
			return []metav1.OwnerReference{}, nil
		}
		c.prunerPodImageFn = func() string { return "docker.io/foo/bar" }

		operatorSpec, _, _, err := c.operatorConfigClient.GetStaticPodOperatorState()
		if err != nil {
			t.Fatalf("unexpected error %q", err)
		}
		failedLimit, succeededLimit := getRevisionLimits(operatorSpec)

		excludedRevisions, err := c.excludedRevisionHistory(operatorStatus, failedLimit, succeededLimit)
		if err != nil {
			t.Fatalf("unexpected error %q", err)
		}
		if apiErr := c.pruneAPIResources(excludedRevisions, excludedRevisions[len(excludedRevisions)-1]); apiErr != nil {
			t.Fatalf("unexpected error %q", apiErr)
		}

		statusConfigMaps, err := c.configMapGetter.ConfigMaps(tc.targetNamespace).List(metav1.ListOptions{})
		if err != nil {
			t.Fatalf("unexpected error %q", err)
		}
		if len(statusConfigMaps.Items) != len(tc.expectedObjects) {
			t.Errorf("expected objects %+v but got %+v", tc.expectedObjects, statusConfigMaps.Items)
		}
	}
}

func TestPruneDiskResources(t *testing.T) {
	tests := []struct {
		name                string
		failedLimit         int32
		succeededLimit      int32
		maxEligibleRevision int
		protectedRevisions  string
		configMaps          []configMapInfo
		expectedErr         string
	}{
		{
			name: "creates prune pod appropriately",
			configMaps: []configMapInfo{
				{
					name:      "revision-status-1",
					namespace: "test",
					revision:  "1",
					phase:     string(v1.PodSucceeded),
				},
				{
					name:      "revision-status-2",
					namespace: "test",
					revision:  "2",
					phase:     string(v1.PodFailed),
				},
				{
					name:      "revision-status-3",
					namespace: "test",
					revision:  "3",
					phase:     string(v1.PodSucceeded),
				},
			},
			maxEligibleRevision: 3,
			protectedRevisions:  "2,3",
			failedLimit:         1,
			succeededLimit:      1,
		},

		{
			name: "defaults to unlimited revision history",
			configMaps: []configMapInfo{
				{
					name:      "revision-status-1",
					namespace: "test",
					revision:  "1",
					phase:     string(v1.PodSucceeded),
				},
				{
					name:      "revision-status-2",
					namespace: "test",
					revision:  "2",
					phase:     string(v1.PodFailed),
				},
				{
					name:      "revision-status-3",
					namespace: "test",
					revision:  "3",
					phase:     string(v1.PodSucceeded),
				},
			},
			maxEligibleRevision: 3,
			protectedRevisions:  "1,2,3",
		},

		{
			name: "protects unknown revision status",
			configMaps: []configMapInfo{
				{
					name:      "revision-status-1",
					namespace: "test",
					revision:  "1",
					phase:     string(v1.PodSucceeded),
				},
				{
					name:      "revision-status-2",
					namespace: "test",
					revision:  "2",
					phase:     "garbage",
				},
			},
			maxEligibleRevision: 2,
			protectedRevisions:  "1,2",
		},
		{
			name: "handles revisions of only one type of phase",
			configMaps: []configMapInfo{
				{
					name:      "revision-status-1",
					namespace: "test",
					revision:  "1",
					phase:     string(v1.PodSucceeded),
				},
				{
					name:      "revision-status-2",
					namespace: "test",
					revision:  "2",
					phase:     string(v1.PodSucceeded),
				},
			},
			maxEligibleRevision: 2,
			protectedRevisions:  "2",
			failedLimit:         1,
			succeededLimit:      1,
		},
		{
			name: "protects all with unlimited revisions",
			configMaps: []configMapInfo{
				{
					name:      "revision-status-1",
					namespace: "test",
					revision:  "1",
					phase:     string(v1.PodSucceeded),
				},
				{
					name:      "revision-status-2",
					namespace: "test",
					revision:  "2",
					phase:     string(v1.PodSucceeded),
				},
			},
			maxEligibleRevision: 2,
			protectedRevisions:  "2",
			failedLimit:         1,
			succeededLimit:      1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()

			var prunerPod *v1.Pod
			kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				prunerPod = action.(ktesting.CreateAction).GetObject().(*v1.Pod)
				return false, nil, nil
			})
			kubeClient.PrependReactor("list", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, configMapList(test.configMaps), nil
			})

			fakeStaticPodOperatorClient := v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					FailedRevisionLimit:    test.failedLimit,
					SucceededRevisionLimit: test.succeededLimit,
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{
					LatestAvailableRevision: 1,
					NodeStatuses: []operatorv1.NodeStatus{
						{
							NodeName:        "test-node-1",
							CurrentRevision: 1,
							TargetRevision:  0,
						},
					},
				},
				nil,
			)
			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &v1.ObjectReference{})

			operatorStatus := &operatorv1.StaticPodOperatorStatus{
				LatestAvailableRevision: 1,
				NodeStatuses: []operatorv1.NodeStatus{
					{
						NodeName:        "test-node-1",
						CurrentRevision: 1,
						TargetRevision:  0,
					},
				},
			}

			c := &PruneController{
				targetNamespace:      "test",
				podResourcePrefix:    "test-pod",
				command:              []string{"/bin/true"},
				configMapGetter:      kubeClient.CoreV1(),
				secretGetter:         kubeClient.CoreV1(),
				podGetter:            kubeClient.CoreV1(),
				eventRecorder:        eventRecorder,
				operatorConfigClient: fakeStaticPodOperatorClient,
			}
			c.ownerRefsFn = func(revision int32) ([]metav1.OwnerReference, error) {
				return []metav1.OwnerReference{}, nil
			}
			c.prunerPodImageFn = func() string { return "docker.io/foo/bar" }

			operatorSpec, _, _, err := c.operatorConfigClient.GetStaticPodOperatorState()
			if err != nil {
				t.Fatalf("unexpected error %q", err)
			}
			failedLimit, succeededLimit := getRevisionLimits(operatorSpec)

			excludedRevisions, err := c.excludedRevisionHistory(operatorStatus, failedLimit, succeededLimit)
			if err != nil {
				t.Fatalf("unexpected error %q", err)
			}
			if diskErr := c.pruneDiskResources(operatorStatus, excludedRevisions, excludedRevisions[len(excludedRevisions)-1]); diskErr != nil {
				t.Fatalf("unexpected error %q", diskErr)
			}

			if prunerPod == nil {
				t.Fatalf("expected to create installer pod")
			}

			if prunerPod.Spec.Containers[0].Image != "docker.io/foo/bar" {
				t.Fatalf("expected docker.io/foo/bar image, got %q", prunerPod.Spec.Containers[0].Image)
			}

			if prunerPod.Spec.Containers[0].Command[0] != "/bin/true" {
				t.Fatalf("expected /bin/true as a command, got %q", prunerPod.Spec.Containers[0].Command[0])
			}

			expectedArgs := []string{
				"-v=4",
				fmt.Sprintf("--max-eligible-revision=%d", test.maxEligibleRevision),
				fmt.Sprintf("--protected-revisions=%s", test.protectedRevisions),
				fmt.Sprintf("--resource-dir=%s", "/etc/kubernetes/static-pod-resources"),
				fmt.Sprintf("--static-pod-name=%s", "test-pod"),
			}

			if len(expectedArgs) != len(prunerPod.Spec.Containers[0].Args) {
				t.Fatalf("expected arguments does not match container arguments: %#v != %#v", expectedArgs, prunerPod.Spec.Containers[0].Args)
			}

			for i, v := range prunerPod.Spec.Containers[0].Args {
				if expectedArgs[i] != v {
					t.Errorf("arg[%d] expected %q, got %q", i, expectedArgs[i], v)
				}
			}
		})
	}
}

func configMapList(configMaps []configMapInfo) *v1.ConfigMapList {
	items := make([]v1.ConfigMap, 0, len(configMaps))
	for _, cm := range configMaps {
		configMap := v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.name,
				Namespace: cm.namespace,
			},
			Data: map[string]string{
				"revision": cm.revision,
				"status":   cm.phase,
			},
		}
		items = append(items, configMap)
	}

	return &v1.ConfigMapList{Items: items}
}
