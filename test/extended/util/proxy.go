package util

import (
	"context"

	v1 "github.com/openshift/api/config/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
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

func GetClusterProxyConfig(oc *rest.Config) *v1.Proxy {
	configClient := configclientset.NewForConfigOrDie(oc)
	proxy, _ := configClient.ConfigV1().Proxies().Get(context.Background(), "cluster", metav1.GetOptions{})

	return proxy
}
