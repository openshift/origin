package server

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateinternalclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	templateservicebroker "github.com/openshift/origin/pkg/templateservicebroker/servicebroker"
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

	clientConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, err
	}
	templateClient, err := templateinternalclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	templateInformers := templateinformer.NewSharedInformerFactory(templateClient, 5*time.Minute)
	templateInformers.Template().InternalVersion().Templates().Informer().AddIndexers(cache.Indexers{
		templateapi.TemplateUIDIndex: func(obj interface{}) ([]string, error) {
			return []string{string(obj.(*templateapi.Template).UID)}, nil
		},
	})

	broker, err := templateservicebroker.NewBroker(
		clientConfig,
		templateInformers.Template().InternalVersion().Templates(),
		c.TemplateNamespaces,
	)
	if err != nil {
		return nil, err
	}

	if err := s.GenericAPIServer.AddPostStartHook("template-service-broker-synctemplates", func(context genericapiserver.PostStartHookContext) error {
		templateInformers.Start(context.StopCh)
		if !controller.WaitForCacheSync("tsb", context.StopCh, templateInformers.Template().InternalVersion().Templates().Informer().HasSynced) {
			return fmt.Errorf("unable to sync caches")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	Route(
		s.GenericAPIServer.Handler.GoRestfulContainer,
		templateapi.ServiceBrokerRoot,
		broker,
	)

	return s, nil
}
