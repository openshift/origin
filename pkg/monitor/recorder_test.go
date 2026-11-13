package monitor

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func pod(uid string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-pod",
			UID:       types.UID(uid),
		},
	}
}

func TestRecordResourceObservedCounts(t *testing.T) {
	tests := []struct {
		name                  string
		recorded              []*corev1.Pod
		expectedUpdateCount   string
		expectedRecreateCount string
	}{
		{
			name:                  "first observation seeds the counts",
			recorded:              []*corev1.Pod{pod("uid-1")},
			expectedUpdateCount:   "1",
			expectedRecreateCount: "0",
		},
		{
			name:                  "repeated observations increment the update count",
			recorded:              []*corev1.Pod{pod("uid-1"), pod("uid-1"), pod("uid-1")},
			expectedUpdateCount:   "3",
			expectedRecreateCount: "0",
		},
		{
			// the UID is part of monitorapi.InstanceKey, so a recreated pod is tracked under its
			// own key and starts counting again from scratch
			name:                  "a recreated pod is tracked separately from its predecessor",
			recorded:              []*corev1.Pod{pod("uid-1"), pod("uid-1"), pod("uid-2")},
			expectedUpdateCount:   "1",
			expectedRecreateCount: "0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := NewRecorder()
			for _, p := range test.recorded {
				recorder.RecordResource("pods", p)
			}

			// the key is derived from the most recently recorded pod, since the UID is part of it
			last := test.recorded[len(test.recorded)-1]
			key := monitorapi.InstanceKey{
				Namespace: last.Namespace,
				Name:      last.Name,
				UID:       string(last.UID),
			}

			stored, ok := recorder.CurrentResourceState()["pods"][key]
			if !ok {
				t.Fatalf("no resource recorded for %v", key)
			}
			annotations := stored.(*corev1.Pod).Annotations

			if actual := annotations[monitorapi.ObservedUpdateCountAnnotation]; actual != test.expectedUpdateCount {
				t.Errorf("expected update count %q, got %q", test.expectedUpdateCount, actual)
			}
			if actual := annotations[monitorapi.ObservedRecreationCountAnnotation]; actual != test.expectedRecreateCount {
				t.Errorf("expected recreation count %q, got %q", test.expectedRecreateCount, actual)
			}
		})
	}
}

// TestRecordResourceDoesNotMutateInput ensures the caller's object is left alone. Callers hand in
// objects straight from an informer cache, so annotating them in place would corrupt the cache.
func TestRecordResourceDoesNotMutateInput(t *testing.T) {
	recorder := NewRecorder()

	first := pod("uid-1")
	recorder.RecordResource("pods", first)
	if first.Annotations != nil {
		t.Errorf("expected the recorded object to be unmodified, got annotations %v", first.Annotations)
	}

	second := pod("uid-1")
	recorder.RecordResource("pods", second)
	if second.Annotations != nil {
		t.Errorf("expected the recorded object to be unmodified, got annotations %v", second.Annotations)
	}
}
