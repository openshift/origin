package cli

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/utils/ptr"
)

func TestReadyPodEndpointCount(t *testing.T) {
	tests := []struct {
		name           string
		endpointSlices []discoveryv1.EndpointSlice
		expected       int
	}{
		{
			name: "entries for unready pods do not count",
			endpointSlices: []discoveryv1.EndpointSlice{{
				Endpoints: []discoveryv1.Endpoint{
					{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}},
					{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-2"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}},
				},
			}},
			expected: 0,
		},
		{
			name: "ready and unset-ready pod endpoints count",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					Endpoints: []discoveryv1.Endpoint{
						{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
						{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-2"}},
					},
				},
				{
					Endpoints: []discoveryv1.Endpoint{
						{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-3"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}},
					},
				},
			},
			expected: 2,
		},
		{
			name: "the same ready pod in multiple slices counts once",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					Endpoints: []discoveryv1.Endpoint{
						{TargetRef: &corev1.ObjectReference{Kind: "Pod", Namespace: "test", Name: "pod-1", UID: "pod-uid"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
					},
				},
				{
					Endpoints: []discoveryv1.Endpoint{
						{TargetRef: &corev1.ObjectReference{Kind: "Pod", Namespace: "test", Name: "pod-1", UID: "pod-uid"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
					},
				},
			},
			expected: 1,
		},
		{
			name: "ready endpoints without pod references do not count",
			endpointSlices: []discoveryv1.EndpointSlice{{
				Endpoints: []discoveryv1.Endpoint{
					{Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
					{TargetRef: &corev1.ObjectReference{Kind: "Node", Name: "node-1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
				},
			}},
			expected: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := readyPodEndpointCount(test.endpointSlices); actual != test.expected {
				t.Fatalf("readyPodEndpointCount() = %d, want %d", actual, test.expected)
			}
		})
	}
}
