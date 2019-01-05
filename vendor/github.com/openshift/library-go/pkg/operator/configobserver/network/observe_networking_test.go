package network

import (
	"reflect"
	"testing"

	"github.com/ghodss/yaml"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestObserveClusterCIDRs(t *testing.T) {
	type Test struct {
		name          string
		config        *corev1.ConfigMap
		expected      []string
		expectedError bool
	}
	tests := []Test{
		{
			"podCIDR, empty old config",
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-config-v1",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"install-config": "networking:\n  podCIDR: podCIDR",
				},
			},
			[]string{"podCIDR"},
			false,
		},
		{
			"podCIDR, existing config",
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-config-v1",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"install-config": "networking:\n  podCIDR: podCIDR",
				},
			},
			[]string{"podCIDR"},
			false,
		},
		{
			"clusterNetworks",
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-config-v1",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"install-config": "networking:\n  clusterNetworks:\n  - cidr: podCIDR1\n  - cidr: podCIDR2",
				},
			},
			[]string{"podCIDR1", "podCIDR2"},
			false,
		},
		{
			"both podCIDR and clusterNetworks",
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-config-v1",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"install-config": "networking:\n  clusterNetworks:\n  - cidr: podCIDR1\n  - cidr: podCIDR2\n  podCIDR: podCIDR",
				},
			},
			[]string{"podCIDR1", "podCIDR2"},
			false,
		},
		{
			"none, no old config",
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-config-v1",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"install-config": "networking: {}\n",
				},
			},
			nil,
			true,
		},
		{
			"none, existing config",
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-config-v1",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"install-config": "networking: {}\n",
				},
			},
			nil,
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := indexer.Add(test.config); err != nil {
				t.Fatal(err.Error())
			}
			result, err := GetClusterCIDRs(corelistersv1.NewConfigMapLister(indexer), events.NewInMemoryRecorder("network"))
			if err != nil && !test.expectedError {
				t.Fatal(err)
			} else if err == nil {
				if test.expectedError {
					t.Fatalf("expected error, but got none")
				}
				if !reflect.DeepEqual(test.expected, result) {
					t.Errorf("\n===== observed config expected:\n%v\n===== observed config actual:\n%v", toYAML(test.expected), toYAML(result))
				}
			}
		})
	}
}

func TestObserveServiceClusterIPRanges(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := indexer.Add(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-config-v1",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"install-config": "networking:\n  serviceCIDR: serviceCIDR",
		},
	}); err != nil {
		t.Fatal(err.Error())
	}
	result, err := GetServiceCIDR(corelistersv1.NewConfigMapLister(indexer), events.NewInMemoryRecorder("network"))
	if err != nil {
		t.Fatal(err)
	}

	if expected := "serviceCIDR"; !reflect.DeepEqual(expected, result) {
		t.Errorf("\n===== observed config expected:\n%v\n===== observed config actual:\n%v", toYAML(expected), toYAML(result))
	}
}

func toYAML(o interface{}) string {
	b, e := yaml.Marshal(o)
	if e != nil {
		return e.Error()
	}
	return string(b)
}
