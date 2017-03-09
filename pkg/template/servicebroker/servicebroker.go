package servicebroker

import (
	"net/http"

	authclient "github.com/openshift/origin/pkg/auth/client"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/controller/shared"
	templateclientset "github.com/openshift/origin/pkg/template/clientset/internalclientset"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

type Broker struct {
	kc                kclientset.Interface
	oc                *client.Client
	templateclient    *templateclientset.Clientset
	restconfig        *rest.Config
	lister            cache.StoreToTemplateLister
	templateNamespace string
}

func NewBroker(restconfig *rest.Config, oc *client.Client, kc kclientset.Interface, informers shared.InformerFactory, templateNamespace string) *Broker {
	return &Broker{
		kc:                kc,
		oc:                oc,
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
