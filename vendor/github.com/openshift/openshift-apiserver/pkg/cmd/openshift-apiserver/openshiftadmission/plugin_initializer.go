package openshiftadmission

import (
	"fmt"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	webhookinitializer "k8s.io/apiserver/pkg/admission/plugin/webhook/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/util/webhook"
	kexternalinformers "k8s.io/client-go/informers"
	kubeclientgoclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/quota/v1/generic"
	"k8s.io/kubernetes/pkg/quota/v1/install"

	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	quotainformer "github.com/openshift/client-go/quota/informers/externalversions"
	securityv1informer "github.com/openshift/client-go/security/informers/externalversions"
	userv1informer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/library-go/pkg/apiserver/admission/admissionrestconfig"
	"github.com/openshift/library-go/pkg/quota/clusterquotamapping"
	"github.com/openshift/openshift-apiserver/pkg/image/apiserver/admission/imagepolicy/originimagereferencemutators"
	"github.com/openshift/openshift-apiserver/pkg/quota/image"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/imagepolicy"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/quota/clusterresourcequota"
)

type InformerAccess interface {
	GetKubernetesInformers() kexternalinformers.SharedInformerFactory
	GetOpenshiftImageInformers() imagev1informer.SharedInformerFactory
	GetOpenshiftQuotaInformers() quotainformer.SharedInformerFactory
	GetOpenshiftSecurityInformers() securityv1informer.SharedInformerFactory
	GetOpenshiftUserInformers() userv1informer.SharedInformerFactory
}

func NewPluginInitializer(
	internalImageRegistryHostname string,
	cloudConfigFile string,
	privilegedLoopbackConfig *rest.Config,
	informers InformerAccess,
	authorizer authorizer.Authorizer,
	restMapper meta.RESTMapper,
	clusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController,
) (admission.PluginInitializer, error) {
	kubeClient, err := kubeclientgoclient.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	imageClient, err := imagev1client.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}

	// TODO make a union registry
	quotaRegistry := generic.NewRegistry(install.NewQuotaConfigurationForAdmission().Evaluators())
	imageEvaluators := image.NewReplenishmentEvaluators(
		nil, // for admission, we never have to list everything, so we can pass nil.
		informers.GetOpenshiftImageInformers().Image().V1().ImageStreams(),
		imageClient.ImageV1(),
	)
	for i := range imageEvaluators {
		quotaRegistry.Add(imageEvaluators[i])
	}

	var cloudConfig []byte
	if len(cloudConfigFile) != 0 {
		var err error
		cloudConfig, err = ioutil.ReadFile(cloudConfigFile)
		if err != nil {
			return nil, fmt.Errorf("error reading from cloud configuration file %s: %v", cloudConfigFile, err)
		}
	}
	// note: we are passing a combined quota registry here...
	genericInitializer := initializer.New(
		kubeClient,
		informers.GetKubernetesInformers(),
		authorizer,
	)
	kubePluginInitializer := kadmission.NewPluginInitializer(
		cloudConfig,
		restMapper,
		generic.NewConfiguration(quotaRegistry.List(), map[schema.GroupResource]struct{}{}))

	webhookAuthResolverWrapper := func(delegate webhook.AuthenticationInfoResolver) webhook.AuthenticationInfoResolver {
		return &webhook.AuthenticationInfoResolverDelegator{
			ClientConfigForFunc: func(server string) (*rest.Config, error) {
				if server == "kubernetes.default.svc" {
					return rest.CopyConfig(privilegedLoopbackConfig), nil
				}
				return delegate.ClientConfigFor(server)
			},
			ClientConfigForServiceFunc: func(serviceName, serviceNamespace string) (*rest.Config, error) {
				if serviceName == "kubernetes" && serviceNamespace == "default" {
					return rest.CopyConfig(privilegedLoopbackConfig), nil
				}
				return delegate.ClientConfigForService(serviceName, serviceNamespace)
			},
		}
	}

	webhookInitializer := webhookinitializer.NewPluginInitializer(
		webhookAuthResolverWrapper,
		aggregatorapiserver.NewClusterIPServiceResolver(informers.GetKubernetesInformers().Core().V1().Services().Lister()),
	)

	return admission.PluginInitializers{
		genericInitializer,
		webhookInitializer,
		kubePluginInitializer,
		imagepolicy.NewInitializer(originimagereferencemutators.OriginImageMutators{}, internalImageRegistryHostname),
		clusterresourcequota.NewInitializer(
			informers.GetOpenshiftQuotaInformers().Quota().V1().ClusterResourceQuotas(),
			clusterQuotaMappingController.GetClusterQuotaMapper(),
			quotaRegistry,
		),
		admissionrestconfig.NewInitializer(*rest.CopyConfig(privilegedLoopbackConfig)),
	}, nil
}
