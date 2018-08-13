package origin

import (
	"time"

	kexternalinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformer "github.com/openshift/client-go/oauth/informers/externalversions"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	routeinformer "github.com/openshift/client-go/route/informers/externalversions"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
)

// informerHolder is a convenient way for us to keep track of the informers, but
// is intentionally private.  We don't want to leak it out further than this package.
// Everything else should say what it wants.
type informerHolder struct {
	internalKubernetesInformers kinternalinformers.SharedInformerFactory
	kubernetesInformers         kexternalinformers.SharedInformerFactory

	// Internal OpenShift informers
	authorizationInformers authorizationinformer.SharedInformerFactory
	imageInformers         imageinformer.SharedInformerFactory
	oauthInformers         oauthinformer.SharedInformerFactory
	quotaInformers         quotainformer.SharedInformerFactory
	routeInformers         routeinformer.SharedInformerFactory
	securityInformers      securityinformer.SharedInformerFactory
	userInformers          userinformer.SharedInformerFactory
}

// NewInformers is only exposed for the build's integration testing until it can be fixed more appropriately.
func NewInformers(clientConfig *rest.Config) (InformerAccess, error) {
	kubeInternal, err := kclientsetinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	kubeExternal, err := kubeclientgoclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	authorizationClient, err := authorizationclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imageclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	quotaClient, err := quotaclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	routerClient, err := routeclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	securityClient, err := securityclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	userClient, err := userclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	// TODO find a single place to create and start informers.  During the 1.7 rebase this will come more naturally in a config object,
	// before then we should try to eliminate our direct to storage access.  It's making us do weird things.
	const defaultInformerResyncPeriod = 10 * time.Minute

	return &informerHolder{
		internalKubernetesInformers: kinternalinformers.NewSharedInformerFactory(kubeInternal, defaultInformerResyncPeriod),
		kubernetesInformers:         kexternalinformers.NewSharedInformerFactory(kubeExternal, defaultInformerResyncPeriod),
		authorizationInformers:      authorizationinformer.NewSharedInformerFactory(authorizationClient, defaultInformerResyncPeriod),
		imageInformers:              imageinformer.NewSharedInformerFactory(imageClient, defaultInformerResyncPeriod),
		oauthInformers:              oauthinformer.NewSharedInformerFactory(oauthClient, defaultInformerResyncPeriod),
		quotaInformers:              quotainformer.NewSharedInformerFactory(quotaClient, defaultInformerResyncPeriod),
		routeInformers:              routeinformer.NewSharedInformerFactory(routerClient, defaultInformerResyncPeriod),
		securityInformers:           securityinformer.NewSharedInformerFactory(securityClient, defaultInformerResyncPeriod),
		userInformers:               userinformer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod),
	}, nil
}

func (i *informerHolder) GetInternalKubernetesInformers() kinternalinformers.SharedInformerFactory {
	return i.internalKubernetesInformers
}
func (i *informerHolder) GetKubernetesInformers() kexternalinformers.SharedInformerFactory {
	return i.kubernetesInformers
}
func (i *informerHolder) GetInternalOpenshiftAuthorizationInformers() authorizationinformer.SharedInformerFactory {
	return i.authorizationInformers
}
func (i *informerHolder) GetInternalOpenshiftImageInformers() imageinformer.SharedInformerFactory {
	return i.imageInformers
}
func (i *informerHolder) GetOpenshiftOauthInformers() oauthinformer.SharedInformerFactory {
	return i.oauthInformers
}
func (i *informerHolder) GetInternalOpenshiftQuotaInformers() quotainformer.SharedInformerFactory {
	return i.quotaInformers
}
func (i *informerHolder) GetOpenshiftRouteInformers() routeinformer.SharedInformerFactory {
	return i.routeInformers
}
func (i *informerHolder) GetInternalOpenshiftSecurityInformers() securityinformer.SharedInformerFactory {
	return i.securityInformers
}
func (i *informerHolder) GetOpenshiftUserInformers() userinformer.SharedInformerFactory {
	return i.userInformers
}

// Start initializes all requested informers.
func (i *informerHolder) Start(stopCh <-chan struct{}) {
	i.internalKubernetesInformers.Start(stopCh)
	i.kubernetesInformers.Start(stopCh)
	i.authorizationInformers.Start(stopCh)
	i.imageInformers.Start(stopCh)
	i.oauthInformers.Start(stopCh)
	i.quotaInformers.Start(stopCh)
	i.routeInformers.Start(stopCh)
	i.securityInformers.Start(stopCh)
	i.userInformers.Start(stopCh)
}
