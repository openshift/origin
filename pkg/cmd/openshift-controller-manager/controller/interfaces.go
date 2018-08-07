package controller

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/meta"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeinformers "k8s.io/client-go/informers"
	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller"

	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	appsinformer "github.com/openshift/client-go/apps/informers/externalversions"
	networkclientinternal "github.com/openshift/client-go/network/clientset/versioned"
	networkinformer "github.com/openshift/client-go/network/informers/externalversions"
	routeinformer "github.com/openshift/client-go/route/informers/externalversions"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	buildclientinternal "github.com/openshift/origin/pkg/build/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclientinternal "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

type ControllerContext struct {
	OpenshiftControllerConfig configapi.OpenshiftControllerConfig

	// ClientBuilder will provide a client for this controller to use
	ClientBuilder ControllerClientBuilder

	KubernetesInformers kubeinformers.SharedInformerFactory

	InternalBuildInformers         buildinformer.SharedInformerFactory
	InternalImageInformers         imageinformer.SharedInformerFactory
	NetworkInformers               networkinformer.SharedInformerFactory
	InternalTemplateInformers      templateinformer.SharedInformerFactory
	InternalQuotaInformers         quotainformer.SharedInformerFactory
	InternalAuthorizationInformers authorizationinformer.SharedInformerFactory
	InternalRouteInformers         routeinformer.SharedInformerFactory
	InternalSecurityInformers      securityinformer.SharedInformerFactory

	AppsInformers appsinformer.SharedInformerFactory

	GenericResourceInformer GenericResourceInformer
	RestMapper              meta.RESTMapper

	// Stop is the stop channel
	Stop <-chan struct{}
	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}
}

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kubeinformers.GenericInformer, error)
	Start(stopCh <-chan struct{})
}

func (c ControllerContext) IsControllerEnabled(name string) bool {
	return controllerapp.IsControllerEnabled(name, sets.String{}, c.OpenshiftControllerConfig.Controllers...)
}

type ControllerClientBuilder interface {
	controller.ControllerClientBuilder
	KubeInternalClient(name string) (kclientsetinternal.Interface, error)
	KubeInternalClientOrDie(name string) kclientsetinternal.Interface

	OpenshiftAppsClient(name string) (appsclient.Interface, error)
	OpenshiftAppsClientOrDie(name string) appsclient.Interface

	OpenshiftInternalBuildClient(name string) (buildclientinternal.Interface, error)
	OpenshiftInternalBuildClientOrDie(name string) buildclientinternal.Interface

	// OpenShift clients based on generated internal clientsets
	OpenshiftInternalTemplateClient(name string) (templateclient.Interface, error)
	OpenshiftInternalTemplateClientOrDie(name string) templateclient.Interface

	OpenshiftInternalImageClient(name string) (imageclientinternal.Interface, error)
	OpenshiftInternalImageClientOrDie(name string) imageclientinternal.Interface

	OpenshiftInternalQuotaClient(name string) (quotaclient.Interface, error)
	OpenshiftInternalQuotaClientOrDie(name string) quotaclient.Interface

	OpenshiftNetworkClient(name string) (networkclientinternal.Interface, error)
	OpenshiftNetworkClientOrDie(name string) networkclientinternal.Interface

	OpenshiftInternalSecurityClient(name string) (securityclient.Interface, error)
	OpenshiftInternalSecurityClientOrDie(name string) securityclient.Interface

	OpenshiftV1SecurityClient(name string) (securityv1client.Interface, error)
	OpenshiftV1SecurityClientOrDie(name string) securityv1client.Interface
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

// OpenshiftAppsClient provides a REST client for the apps API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftAppsClient(name string) (appsclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return appsclient.NewForConfig(clientConfig)
}

// OpenshiftAppsClientOrDie provides a REST client for the apps API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftAppsClientOrDie(name string) appsclient.Interface {
	client, err := b.OpenshiftAppsClient(name)
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

// OpenshiftNetworkClient provides a REST client for the network API.
// If the client cannot be created because of configuration error, this function
// will error.
func (b OpenshiftControllerClientBuilder) OpenshiftNetworkClient(name string) (networkclientinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return networkclientinternal.NewForConfig(clientConfig)
}

// OpenshiftNetworkClientOrDie provides a REST client for the network API.
// If the client cannot be created because of configuration error, this function
// will panic.
func (b OpenshiftControllerClientBuilder) OpenshiftNetworkClientOrDie(name string) networkclientinternal.Interface {
	client, err := b.OpenshiftNetworkClient(name)
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

func (b OpenshiftControllerClientBuilder) OpenshiftV1SecurityClient(name string) (securityv1client.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return securityv1client.NewForConfig(clientConfig)
}

func (b OpenshiftControllerClientBuilder) OpenshiftV1SecurityClientOrDie(name string) securityv1client.Interface {
	client, err := b.OpenshiftV1SecurityClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}
