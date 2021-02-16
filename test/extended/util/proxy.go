package util

import (
	"context"

	v1 "github.com/openshift/api/config/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsClusterProxyEnabled returns true if the cluster has a global proxy enabled
func IsClusterProxyEnabled(oc *CLI) (bool, error) {
	proxy, err := GetClusterProxy(oc)
	if err != nil {
		return false, err
	}
	return len(proxy.Status.HTTPProxy) > 0 || len(proxy.Status.HTTPSProxy) > 0, nil
}

func GetClusterProxy(oc *CLI) (*v1.Proxy, error) {
	proxy := &v1.Proxy{}
	proxy, err := oc.AdminConfigClient().ConfigV1().Proxies().Get(context.Background(), "cluster", metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return proxy, nil
	}
	if err != nil {
		return proxy, err
	}

	return proxy, nil
}
