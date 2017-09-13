package servicebroker

import (
	restclient "k8s.io/client-go/rest"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	extrouteclientset "github.com/openshift/origin/pkg/route/generated/clientset/typed/route/v1"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

// Broker represents the template service broker.  It implements
// openservicebroker/api.Broker.
type Broker struct {
	kc                 kclientset.Interface
	templateclient     internalversiontemplate.TemplateInterface
	extkc              kclientsetexternal.Interface
	extrouteclient     extrouteclientset.RouteV1Interface
	lister             templatelister.TemplateLister
	hasSynced          func() bool
	templateNamespaces map[string]struct{}
}

var _ api.Broker = &Broker{}

func NewBroker(saKubeClientConfig *restclient.Config, informer templateinformer.TemplateInformer, namespaces []string) (*Broker, error) {
	templateNamespaces := map[string]struct{}{}
	for _, namespace := range namespaces {
		templateNamespaces[namespace] = struct{}{}
	}

	internalKubeClient, err := kclientset.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}
	externalKubeClient, err := kclientsetexternal.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}
	extrouteclientset, err := extrouteclientset.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}
	templateClient, err := templateclientset.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}

	b := &Broker{
		kc:                 internalKubeClient,
		extkc:              externalKubeClient,
		extrouteclient:     extrouteclientset,
		templateclient:     templateClient.Template(),
		lister:             informer.Lister(),
		hasSynced:          informer.Informer().HasSynced,
		templateNamespaces: templateNamespaces,
	}

	return b, nil
}
