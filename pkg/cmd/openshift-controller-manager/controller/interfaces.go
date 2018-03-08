package controller

import (
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kexternalinformers "k8s.io/client-go/informers"
	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/controller"

	appinformer "github.com/openshift/origin/pkg/apps/generated/informers/internalversion"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	buildclientinternal "github.com/openshift/origin/pkg/build/generated/internalclientset"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclientinternal "github.com/openshift/origin/pkg/image/generated/internalclientset"
	networkinformer "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclientinternal "github.com/openshift/origin/pkg/network/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

type ControllerContext struct {
	OpenshiftControllerOptions OpenshiftControllerOptions

	EnabledControllers []string

	// ClientBuilder will provide a client for this controller to use
	ClientBuilder ControllerClientBuilder

	ExternalKubeInformers   kexternalinformers.SharedInformerFactory
	InternalKubeInformers   kinternalinformers.SharedInformerFactory
	AppInformers            appinformer.SharedInformerFactory
	BuildInformers          buildinformer.SharedInformerFactory
	ImageInformers          imageinformer.SharedInformerFactory
	NetworkInformers        networkinformer.SharedInformerFactory
	TemplateInformers       templateinformer.SharedInformerFactory
	QuotaInformers          quotainformer.SharedInformerFactory
	AuthorizationInformers  authorizationinformer.SharedInformerFactory
	SecurityInformers       securityinformer.SharedInformerFactory
	GenericResourceInformer GenericResourceInformer

	// Stop is the stop channel
	Stop <-chan struct{}
	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}
}

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kexternalinformers.GenericInformer, error)
	Start(stopCh <-chan struct{})
}

// OpenshiftControllerOptions contain the options used to run the controllers.  Eventually we need to construct a way to properly
// configure these in a config struct.  This at least lets us know what we have.
type OpenshiftControllerOptions struct {
	HPAControllerOptions       HPAControllerOptions
	ResourceQuotaOptions       ResourceQuotaOptions
	ServiceAccountTokenOptions ServiceAccountTokenOptions
}

type HPAControllerOptions struct {
	SyncPeriod               metav1.Duration
	UpscaleForbiddenWindow   metav1.Duration
	DownscaleForbiddenWindow metav1.Duration
}

type ResourceQuotaOptions struct {
	ConcurrentSyncs int32
	SyncPeriod      metav1.Duration
	MinResyncPeriod metav1.Duration
}

type ServiceAccountTokenOptions struct {
	ConcurrentSyncs int32
}

func (c ControllerContext) IsControllerEnabled(name string) bool {
	return controllerapp.IsControllerEnabled(name, sets.String{}, c.EnabledControllers...)
}

type ControllerClientBuilder interface {
	controller.ControllerClientBuilder
	KubeInternalClient(name string) (kclientsetinternal.Interface, error)
	KubeInternalClientOrDie(name string) kclientsetinternal.Interface

	OpenshiftInternalAppsClient(name string) (appsclientinternal.Interface, error)
	OpenshiftInternalAppsClientOrDie(name string) appsclientinternal.Interface

	OpenshiftInternalBuildClient(name string) (buildclientinternal.Interface, error)
	OpenshiftInternalBuildClientOrDie(name string) buildclientinternal.Interface

	// OpenShift clients based on generated internal clientsets
	OpenshiftInternalTemplateClient(name string) (templateclient.Interface, error)
	OpenshiftInternalTemplateClientOrDie(name string) templateclient.Interface

	OpenshiftInternalImageClient(name string) (imageclientinternal.Interface, error)
	OpenshiftInternalImageClientOrDie(name string) imageclientinternal.Interface

	OpenshiftInternalQuotaClient(name string) (quotaclient.Interface, error)
	OpenshiftInternalQuotaClientOrDie(name string) quotaclient.Interface

	OpenshiftInternalNetworkClient(name string) (networkclientinternal.Interface, error)
	OpenshiftInternalNetworkClientOrDie(name string) networkclientinternal.Interface

	OpenshiftInternalSecurityClient(name string) (securityclient.Interface, error)
	OpenshiftInternalSecurityClientOrDie(name string) securityclient.Interface
}

// InitFunc is used to launch a particular controller.  It may run additional "should I activate checks".
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx ControllerContext) (bool, error)

type OpenshiftControllerClientBuilder struct {
	controller.ControllerClientBuilder
}

func (b OpenshiftControllerClientBuilder) KubeInternalClient(name string) (kclientsetinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return kclientsetinternal.NewForConfig(clientConfig)
}

func (b OpenshiftControllerClientBuilder) KubeInternalClientOrDie(name string) kclientsetinternal.Interface {
	client, err := b.KubeInternalClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// OpenshiftInternalTemplateClient provides a REST client for the template API.
// If the client cannot be created because of configuration error, this function
// will return an error.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalTemplateClient(name string) (templateclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return templateclient.NewForConfig(clientConfig)
}

// OpenshiftInternalTemplateClientOrDie provides a REST client for the template API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalTemplateClientOrDie(name string) templateclient.Interface {
	client, err := b.OpenshiftInternalTemplateClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// OpenshiftInternalImageClient provides a REST client for the image API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalImageClient(name string) (imageclientinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return imageclientinternal.NewForConfig(clientConfig)
}

// OpenshiftInternalImageClientOrDie provides a REST client for the image API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalImageClientOrDie(name string) imageclientinternal.Interface {
	client, err := b.OpenshiftInternalImageClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// OpenshiftInternalAppsClient provides a REST client for the apps API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalAppsClient(name string) (appsclientinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return appsclientinternal.NewForConfig(clientConfig)
}

// OpenshiftInternalAppsClientOrDie provides a REST client for the apps API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalAppsClientOrDie(name string) appsclientinternal.Interface {
	client, err := b.OpenshiftInternalAppsClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// OpenshiftInternalBuildClient provides a REST client for the build  API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalBuildClient(name string) (buildclientinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return buildclientinternal.NewForConfig(clientConfig)
}

// OpenshiftInternalBuildClientOrDie provides a REST client for the build API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalBuildClientOrDie(name string) buildclientinternal.Interface {
	client, err := b.OpenshiftInternalBuildClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

func (b OpenshiftControllerClientBuilder) OpenshiftInternalQuotaClient(name string) (quotaclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return quotaclient.NewForConfig(clientConfig)
}

// OpenshiftInternalBuildClientOrDie provides a REST client for the build API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalQuotaClientOrDie(name string) quotaclient.Interface {
	client, err := b.OpenshiftInternalQuotaClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// OpenshiftInternalNetworkClient provides a REST client for the network API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalNetworkClient(name string) (networkclientinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return networkclientinternal.NewForConfig(clientConfig)
}

// OpenshiftInternalNetworkClientOrDie provides a REST client for the network API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalNetworkClientOrDie(name string) networkclientinternal.Interface {
	client, err := b.OpenshiftInternalNetworkClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// OpenshiftInternalSecurityClient provides a REST client for the security API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalSecurityClient(name string) (securityclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return securityclient.NewForConfig(clientConfig)
}

// OpenshiftInternalSecurityClientOrDie provides a REST client for the security API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftInternalSecurityClientOrDie(name string) securityclient.Interface {
	client, err := b.OpenshiftInternalSecurityClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}
