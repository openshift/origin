package certgraphanalysis

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type configMapFilterFunc func(configMap *corev1.ConfigMap) bool

type secretFilterFunc func(configMap *corev1.Secret) bool

type resourceFilteringOptions struct {
	rejectConfigMap configMapFilterFunc
	rejectSecret    secretFilterFunc
}

func (resourceFilteringOptions) approved() {}

var (
	SkipRevisioned = &resourceFilteringOptions{
		rejectConfigMap: func(configMap *corev1.ConfigMap) bool {
			if isRevisioned(configMap.OwnerReferences) {
				return true
			}
			return false
		},
		rejectSecret: func(secret *corev1.Secret) bool {
			if isRevisioned(secret.OwnerReferences) {
				return true
			}
			return false
		},
	}
	SkipHashed = &resourceFilteringOptions{
		rejectConfigMap: func(configMap *corev1.ConfigMap) bool {
			return hasMonitoringHashLabel(configMap.Labels)
		},
		rejectSecret: func(secret *corev1.Secret) bool {
			return hasMonitoringHashLabel(secret.Labels)
		},
	}
)

func isRevisioned(ownerReferences []metav1.OwnerReference) bool {
	for _, curr := range ownerReferences {
		if strings.HasPrefix(curr.Name, "revision-status-") {
			return true
		}
	}

	return false
}

func hasMonitoringHashLabel(labels map[string]string) bool {
	_, ok := labels["monitoring.openshift.io/hash"]
	return ok
}
