package resourcemerge

import (
	corev1 "k8s.io/api/core/v1"
)

func EnsureConfigMap(modified *bool, existing *corev1.ConfigMap, required corev1.ConfigMap) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	// our configmap data always wins.  Users can create their own
	SetMapStringString(modified, &existing.Data, required.Data)
}
