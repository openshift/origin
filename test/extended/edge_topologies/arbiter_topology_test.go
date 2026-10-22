package edge_topologies

import (
	"reflect"
	"testing"

	v1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCountArbiterPodsByNamespace(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		namespaces []string
		want       map[string]int
	}{
		{name: "empty", want: map[string]int{}},
		{name: "two Loki test pods", namespaces: []string{"openshift-e2e-loki", "openshift-e2e-loki"}, want: map[string]int{}},
		{name: "existing e2e namespace", namespaces: []string{"e2e-arbiter-validation"}, want: map[string]int{}},
		{name: "platform counts retained", namespaces: []string{"openshift-etcd", "openshift-etcd", "openshift-dns"}, want: map[string]int{"openshift-etcd": 2, "openshift-dns": 1}},
		{name: "unexpected namespaces retained", namespaces: []string{"application", "openshift-new-component", "openshift-debug-test"}, want: map[string]int{"application": 1, "openshift-new-component": 1, "openshift-debug-test": 1}},
		{name: "prefix near misses retained", namespaces: []string{"e2e", "openshift-e2e", "openshift-e2evil", "team-openshift-e2e-test", "xe2e-test"}, want: map[string]int{"e2e": 1, "openshift-e2e": 1, "openshift-e2evil": 1, "team-openshift-e2e-test": 1, "xe2e-test": 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var pods []corev1.Pod
			for _, namespace := range test.namespaces {
				pods = append(pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}})
			}
			if got := countArbiterPodsByNamespace(pods); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestArbiterPodCountsPreservePlatformExpectations(t *testing.T) {
	t.Parallel()
	for _, platform := range []v1.PlatformType{v1.BareMetalPlatformType, v1.NonePlatformType} {
		t.Run(string(platform), func(t *testing.T) {
			t.Parallel()
			expected := expectedPodsByPlatform[platform]
			var pods []corev1.Pod
			for namespace, count := range expected {
				for index := 0; index < count; index++ {
					pods = append(pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}})
				}
			}
			for _, namespace := range []string{"openshift-e2e-loki", "openshift-e2e-loki", "e2e-arbiter-validation"} {
				pods = append(pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}})
			}
			if got := countArbiterPodsByNamespace(pods); !reflect.DeepEqual(got, expected) {
				t.Fatalf("test pods changed strict platform counts: got %v, want %v", got, expected)
			}
			pods = append(pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-unexpected"}})
			got := countArbiterPodsByNamespace(pods)
			if reflect.DeepEqual(got, expected) || got["openshift-unexpected"] != 1 {
				t.Fatalf("unexpected platform pod must fail strict equality: got %v, want %v", got, expected)
			}
		})
	}
}
