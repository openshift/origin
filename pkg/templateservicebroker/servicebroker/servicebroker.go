package servicebroker

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	restutil "github.com/openshift/origin/pkg/util/rest"
)

// Broker represents the template service broker.  It implements
// openservicebroker/api.Broker.
type Broker struct {
	kc                 kclientset.Interface
	extconfig          *restclient.Config
	templateclient     internalversiontemplate.TemplateInterface
	lister             templatelister.TemplateLister
	hasSynced          func() bool
	templateNamespaces map[string]struct{}
	restmapper         meta.RESTMapper
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
	templateClient, err := templateclientset.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}

	configCopy := *saKubeClientConfig
	configCopy.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: kapi.Codecs}

	b := &Broker{
		kc:                 internalKubeClient,
		extconfig:          &configCopy,
		templateclient:     templateClient.Template(),
		lister:             informer.Lister(),
		hasSynced:          informer.Informer().HasSynced,
		templateNamespaces: templateNamespaces,
		restmapper:         restutil.DefaultMultiRESTMapper(),
	}

	return b, nil
}
