package servicebroker

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	templateclientset "github.com/openshift/client-go/template/clientset/versioned"
	v1template "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	templateinformer "github.com/openshift/client-go/template/informers/externalversions/template/v1"
	templatelister "github.com/openshift/client-go/template/listers/template/v1"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	restutil "github.com/openshift/origin/pkg/util/rest"
)

// Broker represents the template service broker.  It implements
// openservicebroker/api.Broker.
type Broker struct {
	kc                 kclientset.Interface
	extconfig          *restclient.Config
	templateclient     v1template.TemplateV1Interface
	lister             templatelister.TemplateLister
	hasSynced          func() bool
	templateNamespaces map[string]struct{}
	restmapper         meta.RESTMapper
	// TODO - remove this when https://github.com/kubernetes/kubernetes/issues/54940 is fixed
	// a delay between when we create the brokertemplateinstance and the
	// templateinstance.
	gcCreateDelay time.Duration
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
	configCopy.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: legacyscheme.Codecs}

	delay := 5 * time.Second
	value := os.Getenv("TEMPLATE_SERVICE_BROKER_GC_DELAY")
	if len(value) != 0 {
		if v, err := time.ParseDuration(value); err == nil {
			delay = v
		}
	}
	b := &Broker{
		kc:                 internalKubeClient,
		extconfig:          &configCopy,
		templateclient:     templateClient.Template(),
		lister:             informer.Lister(),
		hasSynced:          informer.Informer().HasSynced,
		templateNamespaces: templateNamespaces,
		restmapper:         restutil.DefaultMultiRESTMapper(),
		gcCreateDelay:      delay,
	}

	return b, nil
}
