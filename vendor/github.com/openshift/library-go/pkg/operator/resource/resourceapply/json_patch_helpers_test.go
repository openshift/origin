package resourceapply

import (
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestJSONPatch(t *testing.T) {
	tests := []struct {
		name     string
		original runtime.Object
		modified runtime.Object
		expected string
	}{
		{
			name: "simple diff in pod",
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Annotations: map[string]string{"foo": "bar"}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
			modified: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Annotations: map[string]string{"foo": "nobar"}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
			expected: `{"metadata":{"annotations":{"foo":"nobar"}}}`,
		},
		{
			name: "removing annotation in pod",
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Annotations: map[string]string{"foo": "bar"}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
			modified: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
			expected: `{"metadata":{"annotations":null}}`,
		},
		{
			name: "modified is nil",
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Annotations: map[string]string{"foo": "bar"}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
			expected: `modified object is nil`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if output := JSONPatch(test.original, test.modified); output != test.expected {
				t.Errorf("returned string:\n%s\n\n does not match expected string:\n%s\n", output, test.expected)
			}
		})
	}
}
