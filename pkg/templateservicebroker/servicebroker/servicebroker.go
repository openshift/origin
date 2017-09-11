package servicebroker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

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

// do not wire these back to bootstrappolicy because we're snipping package dependencies
const (
	DefaultOpenShiftInfraNamespace               = "openshift-infra"
	InfraTemplateServiceBrokerServiceAccountName = "template-service-broker"
)

// Broker represents the template service broker.  It implements
// openservicebroker/api.Broker.
type Broker struct {
	// privilegedKubeClientConfig is a privileged clientconfig used for communicating back to the
	// API server.  This is needed because the template service broker is a fake server component that actually
	// needs both the apiserver and controllers to be running
	privilegedKubeClientConfig restclient.Config
	initialize                 sync.Once

	kc                 kclientset.Interface
	templateclient     internalversiontemplate.TemplateInterface
	extkc              kclientsetexternal.Interface
	extrouteclient     extrouteclientset.RouteV1Interface
	lister             templatelister.TemplateLister
	hasSynced          func() bool
	templateNamespaces map[string]struct{}
	ready              chan struct{}
}

var _ api.Broker = &Broker{}

// DeprecatedNewBrokerInsideAPIServer returns a new instance of the template service broker.  While
// built into origin, its initialisation is asynchronous.  This is because it is
// part of the API server, but requires the API server to be up to get its
// service account token.
func DeprecatedNewBrokerInsideAPIServer(privilegedKubeClientConfig restclient.Config, informer templateinformer.TemplateInformer, namespaces []string) *Broker {
	templateNamespaces := map[string]struct{}{}
	for _, namespace := range namespaces {
		templateNamespaces[namespace] = struct{}{}
	}

	b := &Broker{
		privilegedKubeClientConfig: privilegedKubeClientConfig,
		lister:             informer.Lister(),
		hasSynced:          informer.Informer().HasSynced,
		templateNamespaces: templateNamespaces,
		ready:              make(chan struct{}),
	}

	return b
}

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
		ready:              make(chan struct{}),
	}

	// TODO this is an intermediate state. Once we're out of tree, there won't be a ready
	// for now, this skips the hassynced on the lister since we'll be managing that as a poststarthook
	close(b.ready)

	return b, nil
}

// MakeReady actually makes the broker functional
func (b *Broker) MakeReady() error {
	select {
	case <-b.ready:
		return nil
	default:
	}

	// the broker is initialised asynchronously because fetching the service
	// account token requires the main API server to be running.

	glog.V(2).Infof("Template service broker: waiting for authentication token")

	privilegedKubeClient, err := kclientset.NewForConfig(&b.privilegedKubeClientConfig)
	if err != nil {
		return err
	}

	restconfig, kc, extkc, err := Clients(
		b.privilegedKubeClientConfig,
		&ClientLookupTokenRetriever{Client: privilegedKubeClient},
		DefaultOpenShiftInfraNamespace,
		InfraTemplateServiceBrokerServiceAccountName,
	)
	if err != nil {
		return fmt.Errorf("Template service broker: failed to initialize clients: %v", err)
	}

	extrouteclientset, err := extrouteclientset.NewForConfig(restconfig)
	if err != nil {
		return fmt.Errorf("Template service broker: failed to initialize route clientset: %v", err)
	}

	templateclientset, err := templateclientset.NewForConfig(restconfig)
	if err != nil {
		return fmt.Errorf("Template service broker: failed to initialize template clientset: %v", err)
	}

	b.kc = kc
	b.extkc = extkc
	b.extrouteclient = extrouteclientset
	b.templateclient = templateclientset.Template()

	glog.V(2).Infof("Template service broker: waiting for informer sync")

	for !b.hasSynced() {
		time.Sleep(100 * time.Millisecond)
	}

	glog.V(2).Infof("Template service broker: ready; reading namespaces %v", b.templateNamespaces)

	close(b.ready)
	return nil
}

// WaitForReady is called on each incoming API request via a server filter.  It
// is intended to be a quick check that the broker is initialized (which should
// itself be a fast one-off start-up event).
func (b *Broker) WaitForReady() error {
	b.initialize.Do(
		func() {
			if err := b.MakeReady(); err != nil {
				// TODO eventually this will be forward building.  For now, just die if the TSB doesn't actually work and it should
				glog.Fatal(err)
			}
		})

	// delay up to 10 seconds if not ready (unlikely), before returning a
	// "try again" response.
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case <-b.ready:
		return nil
	case <-timer.C:
		return errors.New("timeout waiting for broker to be ready")
	}
}
