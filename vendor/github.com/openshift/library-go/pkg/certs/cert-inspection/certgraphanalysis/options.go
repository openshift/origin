package certgraphanalysis

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type certGenerationOptions struct {
	rejectConfigMap configMapFilterFunc
	rejectSecret    secretFilterFunc
}

var (
	SkipRevisioned = &certGenerationOptions{
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

type certGenerationOptionList []*certGenerationOptions

func (l certGenerationOptionList) rejectConfigMap(configMap *corev1.ConfigMap) bool {
	for _, option := range l {
		if option.rejectConfigMap == nil {
			continue
		}
		if option.rejectConfigMap(configMap) {
			return true
		}
	}
	return false
}

func (l certGenerationOptionList) rejectSecret(secret *corev1.Secret) bool {
	for _, option := range l {
		if option.rejectSecret == nil {
			continue
		}
		if option.rejectSecret(secret) {
			return true
		}
	}
	return false
}

func isRevisioned(ownerReferences []metav1.OwnerReference) bool {
	for _, curr := range ownerReferences {
		if strings.HasPrefix(curr.Name, "revision-status-") {
			return true
		}
	}

	return false
}
