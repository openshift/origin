package deployer

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCategorizePods(t *testing.T) {
	tests := []struct {
		name            string
		pods            []corev1.Pod
		wantGood        []*corev1.Pod
		wantBad         []*corev1.Pod
		wantProgressing bool
		wantErr         bool
	}{
		{"no pod", nil, nil, nil, true, false},
		{
			"good pods, same revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
			}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
			}, nil, false, false,
		},
		{
			"good pods, different revision", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "5"),
			}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				newPod(corev1.PodRunning, corev1.ConditionTrue, "5"),
			}, nil, false, false,
		},
		{
			"ready and unready pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodRunning, corev1.ConditionFalse, "3"),
			}, nil, nil, true, false,
		},
		{
			"good pods and pending pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodPending, corev1.ConditionFalse, "3"),
			}, nil, nil, true, false,
		},
		{
			"good pods and failed pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodFailed, corev1.ConditionFalse, "3"),
			}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
			}, []*corev1.Pod{
				newPod(corev1.PodFailed, corev1.ConditionFalse, "3"),
			}, false, false,
		},
		{
			"good pods and succeeded pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodSucceeded, corev1.ConditionFalse, "3"),
			}, []*corev1.Pod{
				newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
			}, []*corev1.Pod{
				newPod(corev1.PodSucceeded, corev1.ConditionFalse, "3"),
			}, false, false,
		},
		{
			"good pods and unknown phase pods", []corev1.Pod{
				*newPod(corev1.PodRunning, corev1.ConditionTrue, "3"),
				*newPod(corev1.PodUnknown, corev1.ConditionFalse, "3"),
			}, nil, nil, false, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGood, gotBad, gotProgressing, err := categorizePods(tt.pods)
			if (err != nil) != tt.wantErr {
				t.Errorf("categorizePods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotGood, tt.wantGood) {
				t.Errorf("categorizePods() gotGood = %v, want %v", gotGood, tt.wantGood)
			}
			if !reflect.DeepEqual(gotBad, tt.wantBad) {
				t.Errorf("categorizePods() gotBad = %v, want %v", gotBad, tt.wantBad)
			}
			if gotProgressing != tt.wantProgressing {
				t.Errorf("categorizePods() gotProgressing = %v, want %v", gotProgressing, tt.wantProgressing)
			}
		})
	}
}

func newPod(phase corev1.PodPhase, ready corev1.ConditionStatus, revision string) *corev1.Pod {
	pod := corev1.Pod{
		TypeMeta: v1.TypeMeta{Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				"revision": revision,
			}},
		Spec: corev1.PodSpec{},
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
