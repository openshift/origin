package util

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsClusterProxyEnabled returns true if the cluster has a global proxy enabled
func IsClusterProxyEnabled(oc *CLI) (bool, error) {
	proxy, err := oc.AdminConfigClient().ConfigV1().Proxies().Get(context.Background(), "cluster", metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(proxy.Status.HTTPProxy) > 0 || len(proxy.Status.HTTPSProxy) > 0, nil
}
