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
)

func isRevisioned(ownerReferences []metav1.OwnerReference) bool {
	for _, curr := range ownerReferences {
		if strings.HasPrefix(curr.Name, "revision-status-") {
			return true
		}
	}

	return false
}
