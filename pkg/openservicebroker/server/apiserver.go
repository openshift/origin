package server

import (
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion"
	templateservicebroker "github.com/openshift/origin/pkg/template/servicebroker"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

type TemplateServiceBrokerConfig struct {
	GenericConfig *genericapiserver.Config

	KubeClientInternal kclientsetinternal.Interface
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

func (c completedTemplateServiceBrokerConfig) New(delegationTarget genericapiserver.DelegationTarget, stopCh <-chan struct{}) (*TemplateServiceBrokerServer, error) {
	genericServer, err := c.TemplateServiceBrokerConfig.GenericConfig.SkipComplete().New("template-service-broker", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &TemplateServiceBrokerServer{
		GenericAPIServer: genericServer,
	}

	Route(
		s.GenericAPIServer.Handler.GoRestfulContainer,
		templateapi.ServiceBrokerRoot,
		templateservicebroker.NewBroker(
			*c.GenericConfig.LoopbackClientConfig,
			c.KubeClientInternal,
			bootstrappolicy.DefaultOpenShiftInfraNamespace,
			c.TemplateInformers.Template().InternalVersion().Templates(),
			c.TemplateNamespaces,
		),
	)

	// TODO, when/if the TSB becomes a separate entity, this should stop creating the SA and instead die if it cannot find it
	s.GenericAPIServer.AddPostStartHook("template-service-broker-ensure-service-account", func(context genericapiserver.PostStartHookContext) error {
		// TODO jim-minter - this is the spot to create the namespace if needed and create the SA if needed.
		// be tolerant of failures and retry a few times.
		return nil
	})

	return s, nil
}
