package resourcemerge

import (
	corev1 "k8s.io/api/core/v1"
)

func EnsureConfigMap(modified *bool, existing *corev1.ConfigMap, required corev1.ConfigMap) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	MergeMap(modified, &existing.Data, required.Data)
}
