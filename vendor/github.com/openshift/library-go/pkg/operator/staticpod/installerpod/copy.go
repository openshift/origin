package installerpod

import (
	"github.com/golang/glog"
	"golang.org/x/net/context"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/library-go/pkg/operator/resource/retry"
)

// getSecretWithRetry will attempt to get the secret from the API server and retry on any connection errors until
// the context is not done or secret is returned or a HTTP client error is returned.
// In case the optional flag is set, the 404 error is not reported and a nil object is returned instead.
func (o *InstallOptions) getSecretWithRetry(ctx context.Context, name string, isOptional bool) (*v1.Secret, error) {
	var secret *v1.Secret

	err := retry.RetryOnConnectionErrors(ctx, func(ctx context.Context) (bool, error) {
		var clientErr error
		secret, clientErr = o.KubeClient.CoreV1().Secrets(o.Namespace).Get(name, metav1.GetOptions{})
		if clientErr != nil {
			glog.Infof("Failed to get secret %s/%s: %v", o.Namespace, name, clientErr)
			return false, clientErr
		}
		return true, nil
	})

	switch {
	case err == nil:
		glog.Infof("Got secret %s/%s", o.Namespace, name)
		return secret, nil
	case errors.IsNotFound(err) && isOptional:
		return nil, nil
	default:
		return nil, err
	}

}

// getConfigMapWithRetry will attempt to get the configMap from the API server and retry on any connection errors until
// the context is not done or configMap is returned or a HTTP client error is returned.
// In case the optional flag is set, the 404 error is not reported and a nil object is returned instead.
func (o *InstallOptions) getConfigMapWithRetry(ctx context.Context, name string, isOptional bool) (*v1.ConfigMap, error) {
	var config *v1.ConfigMap

	err := retry.RetryOnConnectionErrors(ctx, func(ctx context.Context) (bool, error) {
		var clientErr error
		config, clientErr = o.KubeClient.CoreV1().ConfigMaps(o.Namespace).Get(name, metav1.GetOptions{})
		if clientErr != nil {
			glog.Infof("Failed to get config map %s/%s: %v", o.Namespace, name, clientErr)
			return false, clientErr
		}
		return true, nil
	})

	switch {
	case err == nil:
		glog.Infof("Got configMap %s/%s", o.Namespace, name)
		return config, nil
	case errors.IsNotFound(err) && isOptional:
		return nil, nil
	default:
		return nil, err
	}
}
