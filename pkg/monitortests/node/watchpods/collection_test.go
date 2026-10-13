package watchpods

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func newPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func addIP(in *corev1.Pod, ip string) *corev1.Pod {
	out := in.DeepCopy()
	out.Status.PodIPs = append(out.Status.PodIPs, corev1.PodIP{IP: ip})
	return out
}

func setDeletionTimestamp(in *corev1.Pod, deletionTime time.Time) *corev1.Pod {
	out := in.DeepCopy()
	out.DeletionTimestamp = &metav1.Time{Time: deletionTime}
	return out
}
