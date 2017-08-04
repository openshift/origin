package server

import (
	"fmt"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
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

func (c completedTemplateServiceBrokerConfig) New(delegationTarget genericapiserver.DelegationTarget) (*TemplateServiceBrokerServer, error) {
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
