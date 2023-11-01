package certgraphanalysis

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

type certGenerationOptions struct {
	rejectConfigMap configMapFilterFunc
	rejectSecret    secretFilterFunc
}

var (
	SkipRevisioned = &certGenerationOptions{
		rejectConfigMap: func(configMap *corev1.ConfigMap) bool {
			if metadata, err := meta.Accessor(&configMap); err == nil {
				_, _, revisioned := isRevisioned(metadata)
				return revisioned
			}
			return false
		},
		rejectSecret: func(secret *corev1.Secret) bool {
			if metadata, err := meta.Accessor(&secret); err == nil {
				_, _, revisioned := isRevisioned(metadata)
				return revisioned
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

// returns namespace, name, isRevisioned
func isRevisioned(metadata metav1.Object) (string, string, bool) {
	revisioned := false
	for _, curr := range metadata.GetOwnerReferences() {
		if strings.HasPrefix(curr.Name, "revision-status-") {
			revisioned = true
			break
		}
	}
	if !revisioned {
		return "", "", false
	}
	suffixIndex := strings.LastIndex(metadata.GetName(), "-")
	if suffixIndex < 1 {
		return "", "", false
	}

	return metadata.GetNamespace(), metadata.GetName()[:suffixIndex-1], true
}
