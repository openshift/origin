package servicebroker

import (
	"k8s.io/apiserver/pkg/authentication/user"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	authclient "github.com/openshift/origin/pkg/auth/client"
	"github.com/openshift/origin/pkg/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
)

type Broker struct {
	secretsGetter      internalversion.SecretsGetter
	localSAR           client.LocalSubjectAccessReviewsNamespacer
	templateclient     internalversiontemplate.TemplateInterface
	restconfig         restclient.Config
	lister             templatelister.TemplateLister
	templateNamespaces map[string]struct{}
}

func NewBroker(restconfig restclient.Config, localSAR client.LocalSubjectAccessReviewsNamespacer, secretsGetter internalversion.SecretsGetter, informer templateinformer.TemplateInformer, namespaces []string) *Broker {
	informer.Informer().AddIndexers(cache.Indexers{
		templateapi.TemplateUIDIndex: func(obj interface{}) ([]string, error) {
			return []string{string(obj.(*templateapi.Template).UID)}, nil
		}})

	templateNamespaces := map[string]struct{}{}
	for _, namespace := range namespaces {
		templateNamespaces[namespace] = struct{}{}
	}

	return &Broker{
		secretsGetter:      secretsGetter,
		localSAR:           localSAR,
		templateclient:     templateclientset.NewForConfigOrDie(&restconfig).Template(),
		restconfig:         restconfig,
		lister:             informer.Lister(),
		templateNamespaces: templateNamespaces,
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
