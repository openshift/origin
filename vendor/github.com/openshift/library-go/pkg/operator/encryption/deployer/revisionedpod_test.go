package deployer

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCategorizePods(t *testing.T) {
	tests := []struct {
		name                      string
		pods                      []corev1.Pod
		nodes                     []string
		wantGood                  []*corev1.Pod
		wantBad                   []*corev1.Pod
		wantCategorizeProgressing bool
		wantCategorizeErr         bool

		wantCommonRevision                          string
		wantGetAPIServerRevisionOfAllInstancesError bool
	}{
		{"no pod", nil, nil, nil, nil, true, false, "", false},
		{
			"good pods, same revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node2"),
			}, nil, false, false, "3", false,
		},
		{
			"good pods, different revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "5", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "5", "node2"),
			}, nil, false, false, "", false,
		},
		{
			"ready and unready pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionFalse, "3", "node2"),
			}, []string{"node1", "node2"}, nil, nil, true, false, "3", false,
		},
		{
			"good pods and pending pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodPending, corev1.ConditionFalse, "3", "node2"),
			}, []string{"node1", "node2"}, nil, nil, true, false, "3", false,
		},
		{
			"good pods and failed pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodFailed, corev1.ConditionFalse, "3", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
			}, []*corev1.Pod{
				newPod(corev1.PodFailed, corev1.ConditionFalse, "3", "node2"),
			}, false, false, "3", false,
		},
		{
			"good pods and succeeded pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodSucceeded, corev1.ConditionFalse, "3", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
			}, []*corev1.Pod{
				newPod(corev1.PodSucceeded, corev1.ConditionFalse, "3", "node2"),
			}, false, false, "3", false,
		},
		{
			"good pods and unknown phase pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3", "node1"),
				*newPod(corev1.PodUnknown, corev1.ConditionFalse, "3", "node2"),
			}, []string{"node1", "node2"}, nil, nil, false, true, "", false,
		},
		{
			"all empty revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node2"),
			}, nil, false, false, "0", false,
		},
		{
			"one empty revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "1", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "1", "node2"),
			}, nil, false, false, "1", false,
		},
		{
			"one empty revision, one zero", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "0", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "0", "node2"),
			}, nil, false, false, "0", false,
		},
		{
			"one invalid revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "abc", "node2"),
			}, []string{"node1", "node2"}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "", "node1"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "abc", "node2"),
			}, nil, false, false, "", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGood, gotBad, gotProgressing, err := categorizePods(tt.pods)
			if (err != nil) != tt.wantCategorizeErr {
				t.Errorf("categorizePods() error = %v, wantErr %v", err, tt.wantCategorizeErr)
				return
			}
			if !reflect.DeepEqual(gotGood, tt.wantGood) {
				t.Errorf("categorizePods() gotGood = %v, want %v", gotGood, tt.wantGood)
			}
			if !reflect.DeepEqual(gotBad, tt.wantBad) {
				t.Errorf("categorizePods() gotBad = %v, want %v", gotBad, tt.wantBad)
			}
			if gotProgressing != tt.wantCategorizeProgressing {
				t.Errorf("categorizePods() gotProgressing = %v, want %v", gotProgressing, tt.wantCategorizeProgressing)
			}

			if err != nil {
				rev, err := getAPIServerRevisionOfAllInstances("revision", tt.nodes, tt.pods)
				if (err != nil) != tt.wantCategorizeErr {
					t.Errorf("getAPIServerRevisionOfAllInstances() error = %v, wantErr %v", err, tt.wantGetAPIServerRevisionOfAllInstancesError)
					return
				}
				if rev != tt.wantCommonRevision {
					t.Errorf("getAPIServerRevisionOfAllInstances() rev = %q, want %q", rev, tt.wantCommonRevision)
				}
			}
		})
	}
}

func newPod(phase corev1.PodPhase, ready corev1.ConditionStatus, revision, nodeName string) *corev1.Pod {
	pod := corev1.Pod{
		TypeMeta: v1.TypeMeta{Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				"revision": revision,
			}},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: phase,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: ready,
			}},
		},
	}

	return &pod
}
