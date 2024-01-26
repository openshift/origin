package util

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const ManagedClusterNamespace = "dedicated-admin"

// IsRunningInManagedCluster returns true if the operator is running in a managed cluster.
// It checks for the existence of the dedicated-admin namespace
func IsRunningInManagedCluster(ctx context.Context, c kubernetes.Interface) (bool, error) {
	_, err := c.CoreV1().Namespaces().Get(ctx, ManagedClusterNamespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
