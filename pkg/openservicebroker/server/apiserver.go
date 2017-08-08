package server

import (
	"fmt"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateservicebroker "github.com/openshift/origin/pkg/template/servicebroker"
)

// TODO: this file breaks the layering of pkg/openservicebroker and
// pkg/template/servicebroker; assuming that the latter will move out of origin
// in 3.7, will leave as is for now.

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	Scheme               = runtime.NewScheme()
	Codecs               = serializer.NewCodecFactory(Scheme)

	// if you modify this, make sure you update the crEncoder
	unversionedVersion = schema.GroupVersion{Group: "", Version: "v1"}
	unversionedTypes   = []runtime.Object{
		&metav1.Status{},
		&metav1.WatchEvent{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	}
)

type TemplateServiceBrokerConfig struct {
	GenericConfig *genericapiserver.Config

	// PrivilegedKubeClientConfig is *not* a loopback config, since it needs to point to the kube apiserver
	// TODO remove this and use the SA that start us instead of trying to cyclically find an SA token
	PrivilegedKubeClientConfig restclient.Config

	TemplateInformers  templateinformer.SharedInformerFactory
	TemplateNamespaces []string
}

type TemplateServiceBrokerServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedTemplateServiceBrokerConfig struct {
	*TemplateServiceBrokerConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *TemplateServiceBrokerConfig) Complete() completedTemplateServiceBrokerConfig {
	c.GenericConfig.Complete()

	return completedTemplateServiceBrokerConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *TemplateServiceBrokerConfig) SkipComplete() completedTemplateServiceBrokerConfig {
	return completedTemplateServiceBrokerConfig{c}
}

func (c completedTemplateServiceBrokerConfig) New(delegationTarget genericapiserver.DelegationTarget) (*TemplateServiceBrokerServer, error) {
	genericServer, err := c.TemplateServiceBrokerConfig.GenericConfig.SkipComplete().New("template-service-broker", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &TemplateServiceBrokerServer{
		GenericAPIServer: genericServer,
	}

	broker := templateservicebroker.DeprecatedNewBrokerInsideAPIServer(
		c.PrivilegedKubeClientConfig,
		c.TemplateInformers.Template().InternalVersion().Templates(),
		c.TemplateNamespaces,
	)

	Route(
		s.GenericAPIServer.Handler.GoRestfulContainer,
		templateapi.ServiceBrokerRoot,
		broker,
	)

	// TODO, when the TSB becomes a separate entity, this should stop creating the SA and use its container pod SA identity instead
	s.GenericAPIServer.AddPostStartHook("template-service-broker-ensure-service-account", func(context genericapiserver.PostStartHookContext) error {
		kc, err := kclientsetinternal.NewForConfig(context.LoopbackClientConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("template service broker: failed to get client: %v", err))
			return err
		}

		err = wait.PollImmediate(time.Second, 30*time.Second, func() (done bool, err error) {
			kc.Namespaces().Create(&api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: bootstrappolicy.DefaultOpenShiftInfraNamespace}})

			_, err = kc.ServiceAccounts(bootstrappolicy.DefaultOpenShiftInfraNamespace).Create(&api.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: bootstrappolicy.InfraTemplateServiceBrokerServiceAccountName}})
			switch {
			case err == nil || kapierrors.IsAlreadyExists(err):
				done, err = true, nil
			case kapierrors.IsNotFound(err):
				err = nil
			}

			return
		})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("creation of template-service-broker SA failed: %v", err))
		}
		return err
	})

	return s, nil
}
