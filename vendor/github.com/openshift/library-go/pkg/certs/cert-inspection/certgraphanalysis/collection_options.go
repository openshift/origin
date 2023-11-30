package certgraphanalysis

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type configMapFilterFunc func(configMap *corev1.ConfigMap) bool

type secretFilterFunc func(configMap *corev1.Secret) bool

type resourceFilteringOptions struct {
	rejectConfigMapFn configMapFilterFunc
	rejectSecretFn    secretFilterFunc
}

var (
	_ configMapFilter = &resourceFilteringOptions{}
	_ secretFilter    = &resourceFilteringOptions{}
)

func (*resourceFilteringOptions) approved() {}

func (o *resourceFilteringOptions) rejectConfigMap(configMap *corev1.ConfigMap) bool {
	if o.rejectConfigMapFn == nil {
		return false
	}
	return o.rejectConfigMapFn(configMap)
}

func (o *resourceFilteringOptions) rejectSecret(secret *corev1.Secret) bool {
	if o.rejectSecretFn == nil {
		return false
	}
	return o.rejectSecretFn(secret)
}

var (
	SkipRevisioned = &resourceFilteringOptions{
		rejectConfigMapFn: func(configMap *corev1.ConfigMap) bool {
			return isRevisioned(configMap.OwnerReferences)
		},
		rejectSecretFn: func(secret *corev1.Secret) bool {
			return isRevisioned(secret.OwnerReferences)
		},
	}
	SkipHashed = &resourceFilteringOptions{
		rejectConfigMapFn: func(configMap *corev1.ConfigMap) bool {
			return hasMonitoringHashLabel(configMap.Labels)
		},
		rejectSecretFn: func(secret *corev1.Secret) bool {
			return hasMonitoringHashLabel(secret.Labels)
		},
	}
)

type annotationOptions struct {
	annotationKeys []string
}

// CollectAnnotations creates an option that specifies the list of annotation to collect.
func CollectAnnotations(annotationKeys ...string) *annotationOptions {
	return &annotationOptions{
		annotationKeys: annotationKeys,
	}
}

func (*annotationOptions) approved() {}

func (o *annotationOptions) annotationsToCollect() []string {
	return o.annotationKeys
}

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
