package serving

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func ToServerConfig(servingInfo configv1.HTTPServingInfo, authenticationConfig operatorv1alpha1.DelegatedAuthentication, authorizationConfig operatorv1alpha1.DelegatedAuthorization, kubeConfigFile string) (*genericapiserver.Config, error) {
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

	if !authenticationConfig.Disabled {
		authenticationOptions := genericapiserveroptions.NewDelegatingAuthenticationOptions()
		authenticationOptions.RemoteKubeConfigFile = kubeConfigFile
		if err := authenticationOptions.ApplyTo(&config.Authentication, config.SecureServing, config.OpenAPIConfig); err != nil {
			return nil, err
		}
	}

	if !authorizationConfig.Disabled {
		authorizationOptions := genericapiserveroptions.NewDelegatingAuthorizationOptions()
		authorizationOptions.RemoteKubeConfigFile = kubeConfigFile
		if err := authorizationOptions.ApplyTo(&config.Authorization); err != nil {
			return nil, err
		}
	}

	return config, nil
}
