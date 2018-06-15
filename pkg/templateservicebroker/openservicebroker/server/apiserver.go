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
	"k8s.io/kubernetes/pkg/apis/core/install"
	"k8s.io/kubernetes/pkg/controller"

	templateapiv1 "github.com/openshift/api/template/v1"
	templateclientset "github.com/openshift/client-go/template/clientset/versioned"
	templateinformer "github.com/openshift/client-go/template/informers/externalversions"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
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

func init() {
	install.Install(groupFactoryRegistry, registry, Scheme)

	// we need to add the options to empty v1
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Group: "", Version: "v1"})

	Scheme.AddUnversionedTypes(unversionedVersion, unversionedTypes...)
}

type ExtraConfig struct {
	TemplateNamespaces []string
}

type TemplateServiceBrokerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type TemplateServiceBrokerServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedTemplateServiceBrokerConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *TemplateServiceBrokerConfig) Complete() completedTemplateServiceBrokerConfig {
	cfg := completedTemplateServiceBrokerConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

func (c completedTemplateServiceBrokerConfig) New(delegationTarget genericapiserver.DelegationTarget) (*TemplateServiceBrokerServer, error) {
	genericServer, err := c.GenericConfig.New("template-service-broker", delegationTarget)
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
	templateClient, err := templateclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	templateInformers := templateinformer.NewSharedInformerFactory(templateClient, 5*time.Minute)
	templateInformers.Template().V1().Templates().Informer().AddIndexers(cache.Indexers{
		templateapi.TemplateUIDIndex: func(obj interface{}) ([]string, error) {
			return []string{string(obj.(*templateapiv1.Template).UID)}, nil
		},
	})

	broker, err := templateservicebroker.NewBroker(
		clientConfig,
		templateInformers.Template().V1().Templates(),
		c.ExtraConfig.TemplateNamespaces,
	)
	if err != nil {
		return nil, err
	}

	if err := s.GenericAPIServer.AddPostStartHook("template-service-broker-synctemplates", func(context genericapiserver.PostStartHookContext) error {
		templateInformers.Start(context.StopCh)
		if !controller.WaitForCacheSync("tsb", context.StopCh, templateInformers.Template().V1().Templates().Informer().HasSynced) {
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
