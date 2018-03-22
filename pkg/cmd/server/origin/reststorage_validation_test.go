package origin

import (
	"reflect"
	"testing"
	"time"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	restclient "k8s.io/client-go/rest"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	fakeinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	_ "github.com/openshift/origin/pkg/api/install"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	quotaclientfake "github.com/openshift/origin/pkg/quota/generated/internalclientset/fake"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclientfake "github.com/openshift/origin/pkg/security/generated/internalclientset/fake"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// KnownUpdateValidationExceptions is the list of types that are known to not have an update validation function registered
// If you add something to this list, explain why it doesn't need update validation.
var KnownUpdateValidationExceptions = []reflect.Type{
	reflect.TypeOf(&quotaapi.AppliedClusterResourceQuota{}), // this only retrieved, never created.  its a virtual projection of ClusterResourceQuota
	reflect.TypeOf(&appsapi.DeploymentRequest{}),            // request for deployments already use ValidateDeploymentRequest()
}

// TestValidationRegistration makes sure that any RESTStorage that allows create or update has the correct validation register.
// It doesn't guarantee that it's actually called, but it does guarantee that it at least exists
func TestValidationRegistration(t *testing.T) {
	// TODO figure out how to re-enable this.
	//config := fakeOpenshiftAPIServerConfig()

	//storageMap, err := config.GetRestStorage()
	//if err != nil {
	//	t.Fatal(err)
	//}
	//for key, resourceStorage := range storageMap {
	//	for resource, storage := range resourceStorage {
	//		obj := storage.New()
	//		kindType := reflect.TypeOf(obj)
	//
	//		validationInfo, validatorExists := validation.Validator.GetInfo(obj)
	//
	//		if _, ok := storage.(rest.Creater); ok {
	//			// if we're a creater, then we must have a validate method registered
	//			if !validatorExists {
	//				t.Errorf("No validator registered for %v (used by %s/%s).  Register in pkg/api/validation/register.go.", kindType, resource, key)
	//			}
	//		}
	//
	//		if _, ok := storage.(rest.Updater); ok {
	//			exempted := false
	//			for _, t := range KnownUpdateValidationExceptions {
	//				if t == kindType {
	//					exempted = true
	//					break
	//				}
	//			}
	//
	//			// if we're an updater, then we must have a validateUpdate method registered
	//			if !validatorExists && !exempted {
	//				t.Errorf("No validator registered for %v (used by %s/%s).  Register in pkg/api/validation/register.go.", kindType, resource, key)
	//			}
	//
	//			if !validationInfo.UpdateAllowed && !exempted {
	//				t.Errorf("No validateUpdate method registered for %v (used by %s/%s).  Register in pkg/api/validation/register.go.", kindType, resource, key)
	//			}
	//		}
	//	}
	//}
}

func fakeOpenshiftAPIServerConfig() *OpenshiftAPIConfig {
	internalkubeInformerFactory := kinternalinformers.NewSharedInformerFactory(fakeinternal.NewSimpleClientset(), 1*time.Second)
	quotaInformerFactory := quotainformer.NewSharedInformerFactory(quotaclientfake.NewSimpleClientset(), 0)
	securityInformerFactory := securityinformer.NewSharedInformerFactory(securityclientfake.NewSimpleClientset(), 0)
	restOptionsGetter := restoptions.NewSimpleGetter(&storagebackend.Config{ServerList: []string{"localhost"}})
	sccStorage := sccstorage.NewREST(restOptionsGetter)

	ret := &OpenshiftAPIConfig{
		GenericConfig: &apiserver.RecommendedConfig{
			Config: apiserver.Config{
				LoopbackClientConfig: &restclient.Config{},
				RESTOptionsGetter:    restOptionsGetter,
			},
		},
		ExtraConfig: OpenshiftAPIExtraConfig{
			KubeClientInternal:            &kclientsetinternal.Clientset{},
			KubeletClientConfig:           &kubeletclient.KubeletClientConfig{},
			KubeInternalInformers:         internalkubeInformerFactory,
			QuotaInformers:                quotaInformerFactory,
			SecurityInformers:             securityInformerFactory,
			SCCStorage:                    sccStorage,
			ClusterQuotaMappingController: clusterquotamapping.NewClusterQuotaMappingControllerInternal(internalkubeInformerFactory.Core().InternalVersion().Namespaces(), quotaInformerFactory.Quota().InternalVersion().ClusterResourceQuotas()),
		},
	}
	return ret
}
