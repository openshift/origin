package servicebroker

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/serviceaccounts"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
)

type Broker struct {
	oc                 *client.Client
	kc                 kclientset.Interface
	templateclient     internalversiontemplate.TemplateInterface
	restconfig         restclient.Config
	lister             templatelister.TemplateLister
	templateNamespaces map[string]struct{}
	ready              chan struct{}
}

func NewBroker(privrestconfig restclient.Config, privkc kclientset.Interface, infraNamespace string, informer templateinformer.TemplateInformer, namespaces []string) *Broker {
	templateNamespaces := map[string]struct{}{}
	for _, namespace := range namespaces {
		templateNamespaces[namespace] = struct{}{}
	}

	b := &Broker{
		lister:             informer.Lister(),
		templateNamespaces: templateNamespaces,
		ready:              make(chan struct{}),
	}

	go func() {
		// the broker is initialised asynchronously because fetching the service
		// account token requires the main API server to be running

		glog.Infof("Template service broker: waiting for authentication token")

		restconfig, oc, kc, _, err := serviceaccounts.Clients(
			privrestconfig,
			&serviceaccounts.ClientLookupTokenRetriever{Client: privkc},
			infraNamespace,
			bootstrappolicy.InfraTemplateServiceBrokerServiceAccountName,
		)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Template service broker: failed to initialize: %v", err))
			return
		}

		b.oc = oc
		b.kc = kc
		b.templateclient = templateclientset.NewForConfigOrDie(restconfig).Template()

		glog.Infof("Template service broker: waiting for informer sync")

		for !informer.Informer().HasSynced() {
			time.Sleep(100 * time.Millisecond)
		}

		glog.Infof("Template service broker: ready")

		close(b.ready)
	}()

	return b
}

func (b *Broker) WaitForReady() error {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case <-b.ready:
		return nil
	case <-timer.C:
		return errors.New("timeout waiting for broker to be ready")
	}
}
