package openshiftapiserver

import (
	"time"

	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned"
	authorizationv1informer "github.com/openshift/client-go/authorization/informers/externalversions"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthv1informer "github.com/openshift/client-go/oauth/informers/externalversions"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned"
	quotainformer "github.com/openshift/client-go/quota/informers/externalversions"
	routev1client "github.com/openshift/client-go/route/clientset/versioned"
	routev1informer "github.com/openshift/client-go/route/informers/externalversions"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned"
	securityv1informer "github.com/openshift/client-go/security/informers/externalversions"
	userv1client "github.com/openshift/client-go/user/clientset/versioned"
	userv1informer "github.com/openshift/client-go/user/informers/externalversions"
	kexternalinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
)

// informerHolder is a convenient way for us to keep track of the informers, but
// is intentionally private.  We don't want to leak it out further than this package.
// Everything else should say what it wants.
type InformerHolder struct {
	kubernetesInformers kexternalinformers.SharedInformerFactory

	// Internal OpenShift informers
	authorizationInformers authorizationv1informer.SharedInformerFactory
	imageInformers         imagev1informer.SharedInformerFactory
	oauthInformers         oauthv1informer.SharedInformerFactory
	quotaInformers         quotainformer.SharedInformerFactory
	routeInformers         routev1informer.SharedInformerFactory
	securityInformers      securityv1informer.SharedInformerFactory
	userInformers          userv1informer.SharedInformerFactory
}

// NewInformers is only exposed for the build's integration testing until it can be fixed more appropriately.
func NewInformers(kubeInformers kexternalinformers.SharedInformerFactory, kubeClientConfig *rest.Config, loopbackClientConfig *rest.Config) (*InformerHolder, error) {
	authorizationClient, err := authorizationv1client.NewForConfig(nonProtobufConfig(kubeClientConfig))
	if err != nil {
		return nil, err
	}
	imageClient, err := imagev1client.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthv1client.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	quotaClient, err := quotaclient.NewForConfig(nonProtobufConfig(kubeClientConfig))
	if err != nil {
		return nil, err
	}
	routerClient, err := routev1client.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}
	securityClient, err := securityv1client.NewForConfig(nonProtobufConfig(kubeClientConfig))
	if err != nil {
		return nil, err
	}
	userClient, err := userv1client.NewForConfig(loopbackClientConfig)
	if err != nil {
		return nil, err
	}

	// TODO find a single place to create and start informers.  During the 1.7 rebase this will come more naturally in a config object,
	// before then we should try to eliminate our direct to storage access.  It's making us do weird things.
	const defaultInformerResyncPeriod = 10 * time.Minute

	return &InformerHolder{
		kubernetesInformers:    kubeInformers,
		authorizationInformers: authorizationv1informer.NewSharedInformerFactory(authorizationClient, defaultInformerResyncPeriod),
		imageInformers:         imagev1informer.NewSharedInformerFactory(imageClient, defaultInformerResyncPeriod),
		oauthInformers:         oauthv1informer.NewSharedInformerFactory(oauthClient, defaultInformerResyncPeriod),
		quotaInformers:         quotainformer.NewSharedInformerFactory(quotaClient, defaultInformerResyncPeriod),
		routeInformers:         routev1informer.NewSharedInformerFactory(routerClient, defaultInformerResyncPeriod),
		securityInformers:      securityv1informer.NewSharedInformerFactory(securityClient, defaultInformerResyncPeriod),
		userInformers:          userv1informer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod),
	}, nil
}

// nonProtobufConfig returns a copy of inConfig that doesn't force the use of protobufs,
// for working with CRD-based APIs.
func nonProtobufConfig(inConfig *rest.Config) *rest.Config {
	npConfig := rest.CopyConfig(inConfig)
	npConfig.ContentConfig.AcceptContentTypes = "application/json"
	npConfig.ContentConfig.ContentType = "application/json"
	return npConfig
}

func (i *InformerHolder) GetKubernetesInformers() kexternalinformers.SharedInformerFactory {
	return i.kubernetesInformers
}
func (i *InformerHolder) GetOpenshiftAuthorizationInformers() authorizationv1informer.SharedInformerFactory {
	return i.authorizationInformers
}
func (i *InformerHolder) GetOpenshiftImageInformers() imagev1informer.SharedInformerFactory {
	return i.imageInformers
}
func (i *InformerHolder) GetOpenshiftOauthInformers() oauthv1informer.SharedInformerFactory {
	return i.oauthInformers
}
func (i *InformerHolder) GetOpenshiftQuotaInformers() quotainformer.SharedInformerFactory {
	return i.quotaInformers
}
func (i *InformerHolder) GetOpenshiftRouteInformers() routev1informer.SharedInformerFactory {
	return i.routeInformers
}
func (i *InformerHolder) GetOpenshiftSecurityInformers() securityv1informer.SharedInformerFactory {
	return i.securityInformers
}
func (i *InformerHolder) GetOpenshiftUserInformers() userv1informer.SharedInformerFactory {
	return i.userInformers
}

// Start initializes all requested informers.
func (i *InformerHolder) Start(stopCh <-chan struct{}) {
	i.kubernetesInformers.Start(stopCh)
	i.authorizationInformers.Start(stopCh)
	i.imageInformers.Start(stopCh)
	i.oauthInformers.Start(stopCh)
	i.quotaInformers.Start(stopCh)
	i.routeInformers.Start(stopCh)
	i.securityInformers.Start(stopCh)
	i.userInformers.Start(stopCh)
}
