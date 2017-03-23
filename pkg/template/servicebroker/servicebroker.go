package servicebroker

import (
	"net/http"

	authclient "github.com/openshift/origin/pkg/auth/client"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/controller/shared"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateclientset "github.com/openshift/origin/pkg/template/clientset/internalclientset"
	"k8s.io/kubernetes/pkg/auth/user"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type Broker struct {
	kc                *kclientset.Clientset
	templateclient    *templateclientset.Clientset
	restconfig        *restclient.Config
	lister            cache.StoreToTemplateLister
	templateNamespace string
}

func NewBroker(restconfig *restclient.Config, kc *kclientset.Clientset, informers shared.InformerFactory, templateNamespace string) *Broker {
	return &Broker{
		kc:                kc,
		templateclient:    templateclientset.NewForConfigOrDie(restconfig),
		restconfig:        restconfig,
		lister:            informers.Templates().Lister(),
		templateNamespace: templateNamespace,
	}
}

func (b *Broker) getClientsForUsername(username string) (*kclientset.Clientset, *client.Client, *templateclientset.Clientset, error) {
	restconfigCopy := *b.restconfig
	restconfigCopy.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return authclient.NewImpersonatingRoundTripper(&user.DefaultInfo{Name: username}, b.restconfig.WrapTransport(rt))
	}

	oc, err := client.New(&restconfigCopy)
	if err != nil {
		return nil, nil, nil, err
	}

	kc, err := kclientset.NewForConfig(&restconfigCopy)
	if err != nil {
		return nil, nil, nil, err
	}

	templateclient, err := templateclientset.NewForConfig(&restconfigCopy)
	if err != nil {
		return nil, nil, nil, err
	}

	return kc, oc, templateclient, nil
}

// TODO: UUID should be unique per cluster
var plans = []api.Plan{
	{
		ID:          "7ae2bd88-9b8f-4a17-8014-41a5465c9e71",
		Name:        "default",
		Description: "Default plan",
		Free:        true,
	},
}
