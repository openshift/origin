package util

import (
	"context"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
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

// GetGlobalProxy gets the global proxy configuration from the cluster and returns error
func GetGlobalProxy(oc *CLI) (string, string, string, error) {
	var httpProxy, httpsProxy, noProxy string
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		proxy, err := oc.AdminConfigClient().ConfigV1().Proxies().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Failed to get proxy configuration: %v, retrying...", err)
			return false, nil
		}
		httpProxy = proxy.Status.HTTPProxy
		httpsProxy = proxy.Status.HTTPSProxy
		noProxy = proxy.Status.NoProxy
		return true, nil
	})

	if err != nil {
		// Return empty strings instead of error for clusters without proxy config
		return "", "", "", nil
	}
	return httpProxy, httpsProxy, noProxy, nil
}
