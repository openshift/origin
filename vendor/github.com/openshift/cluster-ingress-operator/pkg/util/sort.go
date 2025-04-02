package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectLess compares two ObjectMeta values and returns true iff the first one
// is less than the second one as determined by their creation timestamps, using
// their UIDs as a tie breaker.
func ObjectLess(x, y *metav1.ObjectMeta) bool {
	if x.CreationTimestamp.Equal(&y.CreationTimestamp) {
		return x.UID < y.UID
	}
	return x.CreationTimestamp.Before(&y.CreationTimestamp)
}
