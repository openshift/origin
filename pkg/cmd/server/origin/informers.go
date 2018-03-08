package origin

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kexternalinformers "k8s.io/client-go/informers"
	kubeclientgoinformers "k8s.io/client-go/informers"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	"github.com/golang/glog"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	routeinformer "github.com/openshift/client-go/route/informers/externalversions"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	appinformer "github.com/openshift/origin/pkg/apps/generated/informers/internalversion"
	appclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appslisters "github.com/openshift/origin/pkg/apps/generated/listers/apps/internalversion"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	networkinformer "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	oauthinformer "github.com/openshift/origin/pkg/oauth/generated/informers/internalversion"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error)
	Start(stopCh <-chan struct{})
}

// genericInternalResourceInformerFunc will return an internal informer for any resource matching
// its group resource, instead of the external version. Only valid for use where the type is accessed
// via generic interfaces, such as the garbage collector with ObjectMeta.
type genericInternalResourceInformerFunc func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error)

func (fn genericInternalResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
	resource.Version = runtime.APIVersionInternal
	return fn(resource)
}

// this is a temporary condition until we rewrite enough of generation to auto-conform to the required interface and no longer need the internal version shim
func (fn genericInternalResourceInformerFunc) Start(stopCh <-chan struct{}) {}

// genericResourceInformerFunc will handle a cast to a matching type
type genericResourceInformerFunc func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error)

func (fn genericResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
	return fn(resource)
}

// this is a temporary condition until we rewrite enough of generation to auto-conform to the required interface and no longer need the internal version shim
func (fn genericResourceInformerFunc) Start(stopCh <-chan struct{}) {}

type genericInformers struct {
	// this is a temporary condition until we rewrite enough of generation to auto-conform to the required interface and no longer need the internal version shim
	startFn func(stopCh <-chan struct{})
	generic []GenericResourceInformer
	// bias is a map that tries loading an informer from another GVR before using the original
	bias map[schema.GroupVersionResource]schema.GroupVersionResource
}

func newGenericInformers(startFn func(stopCh <-chan struct{}), informers ...GenericResourceInformer) genericInformers {
	return genericInformers{
		startFn: startFn,
		generic: informers,
		bias: map[schema.GroupVersionResource]schema.GroupVersionResource{
			{Group: "rbac.authorization.k8s.io", Resource: "rolebindings", Version: "v1beta1"}:        {Group: "rbac.authorization.k8s.io", Resource: "rolebindings", Version: runtime.APIVersionInternal},
			{Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Version: "v1beta1"}: {Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings", Version: runtime.APIVersionInternal},
			{Group: "rbac.authorization.k8s.io", Resource: "roles", Version: "v1beta1"}:               {Group: "rbac.authorization.k8s.io", Resource: "roles", Version: runtime.APIVersionInternal},
			{Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Version: "v1beta1"}:        {Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Version: runtime.APIVersionInternal},
			{Group: "", Resource: "securitycontextconstraints", Version: "v1"}:                        {Group: "", Resource: "securitycontextconstraints", Version: runtime.APIVersionInternal},
		},
	}
}

func (i genericInformers) ForResource(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
	if try, ok := i.bias[resource]; ok {
		if res, err := i.ForResource(try); err == nil {
			return res, nil
		}
	}

	var firstErr error
	for _, generic := range i.generic {
		informer, err := generic.ForResource(resource)
		if err == nil {
			return informer, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	glog.V(4).Infof("Couldn't find informer for %v", resource)
	return nil, firstErr
}

func (i genericInformers) Start(stopCh <-chan struct{}) {
	i.startFn(stopCh)
	for _, generic := range i.generic {
		generic.Start(stopCh)
	}
}

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
	networkInformers       networkinformer.SharedInformerFactory
	oauthInformers         oauthinformer.SharedInformerFactory
	quotaInformers         quotainformer.SharedInformerFactory
	routeInformers         routeinformer.SharedInformerFactory
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
	networkClient, err := networkclient.NewForConfig(clientConfig)
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
		networkInformers:       networkinformer.NewSharedInformerFactory(networkClient, defaultInformerResyncPeriod),
		oauthInformers:         oauthinformer.NewSharedInformerFactory(oauthClient, defaultInformerResyncPeriod),
		quotaInformers:         quotainformer.NewSharedInformerFactory(quotaClient, defaultInformerResyncPeriod),
		routeInformers:         routeinformer.NewSharedInformerFactory(routerClient, defaultInformerResyncPeriod),
		securityInformers:      securityinformer.NewSharedInformerFactory(securityClient, defaultInformerResyncPeriod),
		templateInformers:      templateinformer.NewSharedInformerFactory(templateClient, defaultInformerResyncPeriod),
		userInformers:          userinformer.NewSharedInformerFactory(userClient, defaultInformerResyncPeriod),
	}, nil
}

// AddUserIndexes the API server runs a reverse index on users to groups which requires an index on the group informer
// this activates the lister/watcher, so we want to do it only in this path
func (i *informers) AddUserIndexes() error {
	return i.userInformers.User().V1().Groups().Informer().AddIndexers(cache.Indexers{
		usercache.ByUserIndexName: usercache.ByUserIndexKeys,
	})
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
func (i *informers) GetNetworkInformers() networkinformer.SharedInformerFactory {
	return i.networkInformers
}
func (i *informers) GetOauthInformers() oauthinformer.SharedInformerFactory {
	return i.oauthInformers
}
func (i *informers) GetQuotaInformers() quotainformer.SharedInformerFactory {
	return i.quotaInformers
}
func (i *informers) GetRouteInformers() routeinformer.SharedInformerFactory {
	return i.routeInformers
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
	i.networkInformers.Start(stopCh)
	i.oauthInformers.Start(stopCh)
	i.quotaInformers.Start(stopCh)
	i.routeInformers.Start(stopCh)
	i.securityInformers.Start(stopCh)
	i.templateInformers.Start(stopCh)
	i.userInformers.Start(stopCh)
}

func (i *informers) ToGenericInformer() GenericResourceInformer {
	return newGenericInformers(
		i.Start,
		i.GetExternalKubeInformers(),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetAppInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetAuthorizationInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetBuildInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetImageInformers().ForResource(resource)
		}),
		genericResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetNetworkInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetOauthInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetQuotaInformers().ForResource(resource)
		}),
		genericResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetRouteInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetSecurityInformers().ForResource(resource)
		}),
		genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetTemplateInformers().ForResource(resource)
		}),
		genericResourceInformerFunc(func(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error) {
			return i.GetUserInformers().ForResource(resource)
		}),
	)
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
