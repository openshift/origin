package controller

import (
	"github.com/golang/glog"

	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/controller"

	appinformer "github.com/openshift/origin/pkg/apps/generated/informers/internalversion"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	osclient "github.com/openshift/origin/pkg/client"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclientinternal "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

type ControllerContext struct {
	KubeControllerContext kubecontroller.ControllerContext

	// ClientBuilder will provide a client for this controller to use
	ClientBuilder ControllerClientBuilder

	ExternalKubeInformers  kexternalinformers.SharedInformerFactory
	InternalKubeInformers  kinternalinformers.SharedInformerFactory
	AppInformers           appinformer.SharedInformerFactory
	BuildInformers         buildinformer.SharedInformerFactory
	ImageInformers         imageinformer.SharedInformerFactory
	TemplateInformers      templateinformer.SharedInformerFactory
	QuotaInformers         quotainformer.SharedInformerFactory
	AuthorizationInformers authorizationinformer.SharedInformerFactory
	SecurityInformers      securityinformer.SharedInformerFactory

	// Stop is the stop channel
	Stop <-chan struct{}
}

// TODO wire this up to something that handles the names.  The logic is available upstream, we just have to wire to it
func (c ControllerContext) IsControllerEnabled(name string) bool {
	return true
}

type ControllerClientBuilder interface {
	controller.ControllerClientBuilder
	KubeInternalClient(name string) (kclientsetinternal.Interface, error)
	KubeInternalClientOrDie(name string) kclientsetinternal.Interface

	// Legacy OpenShift client (pkg/client)
	DeprecatedOpenshiftClient(name string) (osclient.Interface, error)
	DeprecatedOpenshiftClientOrDie(name string) osclient.Interface

	// OpenShift clients based on generated internal clientsets
	OpenshiftInternalTemplateClient(name string) (templateclient.Interface, error)
	OpenshiftInternalTemplateClientOrDie(name string) templateclient.Interface

	OpenshiftInternalImageClient(name string) (imageclientinternal.Interface, error)
	OpenshiftInternalImageClientOrDie(name string) imageclientinternal.Interface

	OpenshiftInternalAppsClient(name string) (appsclientinternal.Interface, error)
	OpenshiftInternalAppsClientOrDie(name string) appsclientinternal.Interface
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

func (b OpenshiftControllerClientBuilder) DeprecatedOpenshiftClient(name string) (osclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return osclient.New(clientConfig)
}

func (b OpenshiftControllerClientBuilder) DeprecatedOpenshiftClientOrDie(name string) osclient.Interface {
	client, err := b.DeprecatedOpenshiftClient(name)
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

// FromKubeInitFunc adapts a kube init func to an openshift one
func FromKubeInitFunc(initFn kubecontroller.InitFunc) InitFunc {
	return func(ctx ControllerContext) (bool, error) {
		return initFn(ctx.KubeControllerContext)
	}
}
