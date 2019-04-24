package serving

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/klog"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func ToServerConfig(ctx context.Context, servingInfo configv1.HTTPServingInfo, authenticationConfig operatorv1alpha1.DelegatedAuthentication, authorizationConfig operatorv1alpha1.DelegatedAuthorization,
	kubeConfigFile string) (*genericapiserver.Config, error) {
	scheme := runtime.NewScheme()
	metav1.AddToGroupVersion(scheme, metav1.SchemeGroupVersion)
	config := genericapiserver.NewConfig(serializer.NewCodecFactory(scheme))

	servingOptions, err := ToServingOptions(servingInfo)
	if err != nil {
		return nil, err
	}

	if err := servingOptions.ApplyTo(&config.SecureServing, &config.LoopbackClientConfig); err != nil {
		return nil, err
	}

	var lastApplyErr error

	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if !authenticationConfig.Disabled {
		authenticationOptions := genericapiserveroptions.NewDelegatingAuthenticationOptions()
		authenticationOptions.RemoteKubeConfigFile = kubeConfigFile

		// In some cases the API server can return connection refused when getting the "extension-apiserver-authentication"
		// config map.
		err := wait.PollImmediateUntil(1*time.Second, func() (done bool, err error) {
			lastApplyErr = authenticationOptions.ApplyTo(&config.Authentication, config.SecureServing, config.OpenAPIConfig)
			if lastApplyErr != nil {
				klog.V(4).Infof("Error initializing delegating authentication (will retry): %v", err)
				return false, nil
			}
			return true, nil
		}, pollCtx.Done())
		if err != nil {
			return nil, lastApplyErr
		}
	}

	if !authorizationConfig.Disabled {
		authorizationOptions := genericapiserveroptions.NewDelegatingAuthorizationOptions()
		authorizationOptions.RemoteKubeConfigFile = kubeConfigFile

		// In some cases the API server can return connection refused when getting the "extension-apiserver-authentication"
		// config map.
		err := wait.PollImmediateUntil(1*time.Second, func() (done bool, err error) {
			lastApplyErr = authorizationOptions.ApplyTo(&config.Authorization)
			if lastApplyErr != nil {
				klog.V(4).Infof("Error initializing delegating authorization (will retry): %v", err)
				return false, nil
			}
			return true, nil
		}, pollCtx.Done())
		if err != nil {
			return nil, lastApplyErr
		}
	}

	return config, nil
}
