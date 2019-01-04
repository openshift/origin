package prune

import (
	"fmt"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
)

type configMapInfo struct {
	name      string
	namespace string
	revision  string
	phase     string
}

func TestPruneRevisionHistory(t *testing.T) {
	tests := []struct {
		name           string
		failedLimit    int
		succeededLimit int
		maxEligibleID  int
		protectedIDs   string
		configMaps     []configMapInfo
		expectedErr    string
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
			maxEligibleID:  3,
			protectedIDs:   "2,3",
			failedLimit:    1,
			succeededLimit: 1,
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
			maxEligibleID: 3,
			protectedIDs:  "1,2,3",
		},

		{
			name: "returns an error for unknown revision status",
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
			maxEligibleID: 2,
			protectedIDs:  "1,2",
			expectedErr:   "unknown pod status phase for revision 2: garbage",
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
			maxEligibleID:  2,
			protectedIDs:   "2",
			failedLimit:    1,
			succeededLimit: 1,
		},
	}

	for _, test := range tests {
		kubeClient := fake.NewSimpleClientset()

		var prunerPod *v1.Pod
		kubeClient.PrependReactor("create", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			prunerPod = action.(ktesting.CreateAction).GetObject().(*v1.Pod)
			return false, nil, nil
		})
		kubeClient.PrependReactor("list", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, configMapList(test.configMaps), nil
		})

		fakeStaticPodOperatorClient := common.NewFakeStaticPodOperatorClient(
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

		operatorStatus := &operatorv1.StaticPodOperatorStatus{
			LatestAvailableRevision: 1,
			NodeStatuses: []operatorv1.NodeStatus{
				{
					NodeName:        "test-node-1",
					CurrentRevision: 0,
					TargetRevision:  0,
				},
			},
		}

		c := &PruneController{
			targetNamespace:        "test",
			podResourcePrefix:      "test-pod",
			command:                []string{"/bin/true"},
			kubeClient:             kubeClient,
			eventRecorder:          eventRecorder,
			operatorConfigClient:   fakeStaticPodOperatorClient,
			failedRevisionLimit:    test.failedLimit,
			succeededRevisionLimit: test.succeededLimit,
		}
		c.prunerPodImageFn = func() string { return "docker.io/foo/bar" }

		err := c.pruneRevisionHistory(operatorStatus)
		if err != nil {
			if err.Error() != test.expectedErr {
				t.Errorf("expected error %v, got %v", test.expectedErr, err)
			}

			if prunerPod != nil {
				t.Fatalf("expected not to create installer pod")
			}
			continue
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
			fmt.Sprintf("--max-eligible-id=%d", test.maxEligibleID),
			fmt.Sprintf("--protected-ids=%s", test.protectedIDs),
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
				"phase":    cm.phase,
			},
		}
		items = append(items, configMap)
	}

	return &v1.ConfigMapList{Items: items}
}
