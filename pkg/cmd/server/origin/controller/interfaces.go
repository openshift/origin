package controller

import (
	"github.com/golang/glog"

	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/controller"

	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	buildinformer "github.com/openshift/origin/pkg/build/generated/informers/internalversion"
	osclient "github.com/openshift/origin/pkg/client"
	appinformer "github.com/openshift/origin/pkg/deploy/generated/informers/internalversion"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
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
	DeprecatedOpenshiftClient(name string) (osclient.Interface, error)
	DeprecatedOpenshiftClientOrDie(name string) osclient.Interface
	OpenshiftTemplateClient(name string) (templateclient.Interface, error)
	OpenshiftTemplateClientOrDie(name string) templateclient.Interface
	OpenshiftImageClient(name string) (imageclient.Interface, error)
	OpenshiftImageClientOrDie(name string) imageclient.Interface
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

func (b OpenshiftControllerClientBuilder) OpenshiftTemplateClient(name string) (templateclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return templateclient.NewForConfig(clientConfig)
}

func (b OpenshiftControllerClientBuilder) OpenshiftTemplateClientOrDie(name string) templateclient.Interface {
	client, err := b.OpenshiftTemplateClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

func (b OpenshiftControllerClientBuilder) OpenshiftImageClient(name string) (imageclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return imageclient.NewForConfig(clientConfig)
}

func (b OpenshiftControllerClientBuilder) OpenshiftImageClientOrDie(name string) imageclient.Interface {
	client, err := b.OpenshiftImageClient(name)
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
