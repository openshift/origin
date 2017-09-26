package start

import (
	"time"

	kubeclientgoinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	appinformer "github.com/openshift/origin/pkg/apps/generated/informers/internalversion"
	appclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appslisters "github.com/openshift/origin/pkg/apps/generated/listers/apps/internalversion"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	userinformer "github.com/openshift/origin/pkg/user/generated/informers/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

// informers is a convenient way for us to keep track of the informers, but
// is intentionally private.  We don't want to leak it out further than this package.
// Everything else should say what it wants.
type informers struct {
	internalKubeInformers  kinternalinformers.SharedInformerFactory
	externalKubeInformers  kexternalinformers.SharedInformerFactory
	clientGoKubeInformers  kubeclientgoinformers.SharedInformerFactory
	appInformers           appinformer.SharedInformerFactory
	authorizationInformers authorizationinformer.SharedInformerFactory
	buildInformers         buildinformer.SharedInformerFactory
	imageInformers         imageinformer.SharedInformerFactory
	quotaInformers         quotainformer.SharedInformerFactory
	securityInformers      securityinformer.SharedInformerFactory
	templateInformers      templateinformer.SharedInformerFactory
	userInformers          userinformer.SharedInformerFactory
}

// NewInformers is only exposed for the build's integration testing until it can be fixed more appropriately.
func NewInformers(options configapi.MasterConfig) (*informers, error) {
	clientConfig, kubeInternal, kubeExternal, kubeClientGoExternal, err := getAllClients(options)
	if err != nil {
		return nil, err
	}

	appClient, err := appclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	authorizationClient, err := authorizationclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	buildClient, err := buildclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imageclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	quotaClient, err := quotaclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	securityClient, err := securityclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	templateClient, err := templateclient.NewForConfig(clientConfig)
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

	appInformers := appinformer.NewSharedInformerFactory(appClient, defaultInformerResyncPeriod)
	appInformers.Apps().InternalVersion().DeploymentConfigs().Informer().AddIndexers(
		map[string]cache.IndexFunc{appslisters.ImageStreamReferenceIndex: appslisters.ImageStreamReferenceIndexFunc})

	return &informers{
		internalKubeInformers:  kinternalinformers.NewSharedInformerFactory(kubeInternal, defaultInformerResyncPeriod),
		externalKubeInformers:  kexternalinformers.NewSharedInformerFactory(kubeExternal, defaultInformerResyncPeriod),
		clientGoKubeInformers:  kubeclientgoinformers.NewSharedInformerFactory(kubeClientGoExternal, defaultInformerResyncPeriod),
		appInformers:           appInformers,
		authorizationInformers: authorizationinformer.NewSharedInformerFactory(authorizationClient, defaultInformerResyncPeriod),
		buildInformers:         buildinformer.NewSharedInformerFactory(buildClient, defaultInformerResyncPeriod),
		imageInformers:         imageinformer.NewSharedInformerFactory(imageClient, defaultInformerResyncPeriod),
		quotaInformers:         quotainformer.NewSharedInformerFactory(quotaClient, defaultInformerResyncPeriod),
		securityInformers:      securityinformer.NewSharedInformerFactory(securityClient, defaultInformerResyncPeriod),
		templateInformers:      templateinformer.NewSharedInformerFactory(templateClient, defaultInformerResyncPeriod),
		userInformers:          userinformer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod),
	}, nil
}

func (i *informers) GetInternalKubeInformers() kinternalinformers.SharedInformerFactory {
	return i.internalKubeInformers
}
func (i *informers) GetExternalKubeInformers() kexternalinformers.SharedInformerFactory {
	return i.externalKubeInformers
}
func (i *informers) GetClientGoKubeInformers() kubeclientgoinformers.SharedInformerFactory {
	return i.clientGoKubeInformers
}
func (i *informers) GetAppInformers() appinformer.SharedInformerFactory {
	return i.appInformers
}
func (i *informers) GetAuthorizationInformers() authorizationinformer.SharedInformerFactory {
	return i.authorizationInformers
}
func (i *informers) GetBuildInformers() buildinformer.SharedInformerFactory {
	return i.buildInformers
}
func (i *informers) GetImageInformers() imageinformer.SharedInformerFactory {
	return i.imageInformers
}
func (i *informers) GetQuotaInformers() quotainformer.SharedInformerFactory {
	return i.quotaInformers
}
func (i *informers) GetSecurityInformers() securityinformer.SharedInformerFactory {
	return i.securityInformers
}
func (i *informers) GetTemplateInformers() templateinformer.SharedInformerFactory {
	return i.templateInformers
}
func (i *informers) GetUserInformers() userinformer.SharedInformerFactory {
	return i.userInformers
}

// Start initializes all requested informers.
func (i *informers) Start(stopCh <-chan struct{}) {
	i.internalKubeInformers.Start(stopCh)
	i.externalKubeInformers.Start(stopCh)
	i.clientGoKubeInformers.Start(stopCh)
	i.appInformers.Start(stopCh)
	i.authorizationInformers.Start(stopCh)
	i.buildInformers.Start(stopCh)
	i.imageInformers.Start(stopCh)
	i.quotaInformers.Start(stopCh)
	i.securityInformers.Start(stopCh)
	i.templateInformers.Start(stopCh)
	i.userInformers.Start(stopCh)
}

func getAllClients(options configapi.MasterConfig) (*rest.Config, kclientsetinternal.Interface, kclientsetexternal.Interface, kubeclientgoclient.Interface, error) {
	kubeInternal, clientConfig, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kubeExternal, _, err := configapi.GetExternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kubeClientGoClientSet, err := kubeclientgoclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return clientConfig, kubeInternal, kubeExternal, kubeClientGoClientSet, nil
}
