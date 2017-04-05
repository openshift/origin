package servicebroker

import (
	"k8s.io/kubernetes/pkg/auth/user"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/restclient"

	authclient "github.com/openshift/origin/pkg/auth/client"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/controller/shared"
	templateclientset "github.com/openshift/origin/pkg/template/clientset/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/clientset/internalclientset/typed/template/internalversion"
)

type Broker struct {
	secretsGetter     internalversion.SecretsGetter
	localSAR          client.LocalSubjectAccessReviewsNamespacer
	templateclient    internalversiontemplate.TemplateInterface
	restconfig        restclient.Config
	lister            cache.StoreToTemplateLister
	templateNamespace string
}

func NewBroker(restconfig restclient.Config, localSAR client.LocalSubjectAccessReviewsNamespacer, secretsGetter internalversion.SecretsGetter, informers shared.InformerFactory, templateNamespace string) *Broker {
	return &Broker{
		secretsGetter:     secretsGetter,
		localSAR:          localSAR,
		templateclient:    templateclientset.NewForConfigOrDie(&restconfig).Template(),
		restconfig:        restconfig,
		lister:            informers.Templates().Lister(),
		templateNamespace: templateNamespace,
	}
}

func (b *Broker) getClientsForUsername(username string) (kclientset.Interface, client.Interface, templateclientset.Interface, error) {
	u := &user.DefaultInfo{Name: username}

	oc, err := authclient.NewImpersonatingOpenShiftClient(u, b.restconfig)
	if err != nil {
		return nil, nil, nil, err
	}

	kc, err := authclient.NewImpersonatingKubernetesClientset(u, b.restconfig)
	if err != nil {
		return nil, nil, nil, err
	}

	impersonatingConfig := authclient.NewImpersonatingConfig(u, b.restconfig)
	templateclient, err := templateclientset.NewForConfig(&impersonatingConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	return kc, oc, templateclient, nil
}
