package serving

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func ToServerConfig(ctx context.Context, servingInfo configv1.HTTPServingInfo, authenticationConfig operatorv1alpha1.DelegatedAuthentication, authorizationConfig operatorv1alpha1.DelegatedAuthorization,
	kubeConfigFile string, kubeClient *kubernetes.Clientset, le *configv1.LeaderElection) (*genericapiserver.Config, error) {
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

	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if !authenticationConfig.Disabled {
		authenticationOptions := genericapiserveroptions.NewDelegatingAuthenticationOptions()
		authenticationOptions.RemoteKubeConfigFile = kubeConfigFile
		// the platform generally uses 30s for /metrics scraping, avoid API request for every other /metrics request to the component
		authenticationOptions.CacheTTL = 35 * time.Second

		// In some cases the API server can return connection refused when getting the "extension-apiserver-authentication"
		// config map.
		if le != nil && !le.Disable {
			err := assertAPIConnection(pollCtx, kubeClient, le)
			if err != nil {
				return nil, fmt.Errorf("failed checking apiserver connectivity: %w", err)
			}
		}

		err = authenticationOptions.ApplyTo(&config.Authentication, config.SecureServing, config.OpenAPIConfig)
		if err != nil {
			return nil, fmt.Errorf("error initializing delegating authentication: %w", err)
		}
	}

	if !authorizationConfig.Disabled {
		authorizationOptions := genericapiserveroptions.NewDelegatingAuthorizationOptions().
			WithAlwaysAllowPaths("/healthz", "/readyz", "/livez"). // this allows the kubelet to always get health and readiness without causing an access check
			WithAlwaysAllowGroups("system:masters")                // in a kube cluster, system:masters can take any action, so there is no need to ask for an authz check

		authorizationOptions.RemoteKubeConfigFile = kubeConfigFile
		// the platform generally uses 30s for /metrics scraping, avoid API request for every other /metrics request to the component
		authorizationOptions.AllowCacheTTL = 35 * time.Second

		// In some cases the API server can return connection refused when getting the "extension-apiserver-authentication"
		// config map.
		if le != nil && !le.Disable {
			err := assertAPIConnection(pollCtx, kubeClient, le)
			if err != nil {
				return nil, fmt.Errorf("failed checking connectivity: %w", err)
			}
		}

		err := authorizationOptions.ApplyTo(&config.Authorization)
		if err != nil {
			return nil, fmt.Errorf("error initializing delegating authentication: %w", err)
		}
	}

	return config, nil
}

func assertAPIConnection(ctx context.Context, kubeClient *kubernetes.Clientset, le *configv1.LeaderElection) error {
	var lastErr error
	err := wait.PollImmediateUntil(1*time.Second, func() (done bool, err error) {
		_, lastErr = kubeClient.CoordinationV1().Leases(le.Namespace).Get(ctx, le.Name, metav1.GetOptions{})
		if lastErr != nil && !apierrors.IsNotFound(lastErr) {
			klog.V(4).Infof("Error checking for connectivity to apiserver (GET lease %s/%s): %v", le.Namespace, le.Name, lastErr)
			return false, nil
		}
		return true, nil
	}, ctx.Done())
	if err != nil {
		return lastErr
	}

	return nil
}
