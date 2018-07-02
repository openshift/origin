package servicebroker

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	kclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	templateclientset "github.com/openshift/client-go/template/clientset/versioned"
	v1template "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	templateinformer "github.com/openshift/client-go/template/informers/externalversions/template/v1"
	templatelister "github.com/openshift/client-go/template/listers/template/v1"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

// Broker represents the template service broker.  It implements
// openservicebroker/api.Broker.
type Broker struct {
	kc                 kclientset.Interface
	templateclient     v1template.TemplateV1Interface
	lister             templatelister.TemplateLister
	hasSynced          func() bool
	templateNamespaces map[string]struct{}
	restmapper         meta.RESTMapper
	dynamicClient      dynamic.Interface

	// TODO - remove this when https://github.com/kubernetes/kubernetes/issues/54940 is fixed
	// a delay between when we create the brokertemplateinstance and the
	// templateinstance.
	gcCreateDelay time.Duration
}

var _ api.Broker = &Broker{}

// NewBroker - returns a new Broker
func NewBroker(saKubeClientConfig *restclient.Config, informer templateinformer.TemplateInformer, namespaces []string) (*Broker, error) {
	templateNamespaces := map[string]struct{}{}
	for _, namespace := range namespaces {
		templateNamespaces[namespace] = struct{}{}
	}

	kubeClient, err := kclientset.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}
	templateClient, err := templateclientset.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(saKubeClientConfig)
	if err != nil {
		return nil, err
	}

	// TODO use the genericclioptions and use "normal" construction
	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	discoveryConfig := restclient.CopyConfig(saKubeClientConfig)
	discoveryConfig.Burst = 100
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(discoveryConfig)
	if err != nil {
		return nil, err
	}
	cachedDiscovery := cacheddiscovery.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)

	// start keeping the restmapper up to date with reality
	restMapper.Reset()
	go wait.Until(restMapper.Reset, 30*time.Second, wait.NeverStop)

	delay := 5 * time.Second
	value := os.Getenv("TEMPLATE_SERVICE_BROKER_GC_DELAY")
	if len(value) != 0 {
		if v, err := time.ParseDuration(value); err == nil {
			delay = v
		}
	}
	b := &Broker{
		kc:                 kubeClient,
		templateclient:     templateClient.Template(),
		lister:             informer.Lister(),
		hasSynced:          informer.Informer().HasSynced,
		templateNamespaces: templateNamespaces,
		restmapper:         restMapper,
		dynamicClient:      dynamicClient,
		gcCreateDelay:      delay,
	}

	return b, nil
}
