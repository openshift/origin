package origin

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/rest"
	extapi "k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/controller/informers"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/storage/storagebackend"

	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/api/validation"
	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/controller/shared"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// KnownUpdateValidationExceptions is the list of types that are known to not have an update validation function registered
// If you add something to this list, explain why it doesn't need update validation.
var KnownUpdateValidationExceptions = []reflect.Type{
	reflect.TypeOf(&extapi.Scale{}),                         // scale operation uses the ValidateScale() function for both create and update
	reflect.TypeOf(&quotaapi.AppliedClusterResourceQuota{}), // this only retrieved, never created.  its a virtual projection of ClusterResourceQuota
	reflect.TypeOf(&deployapi.DeploymentRequest{}),          // request for deployments already use ValidateDeploymentRequest()
}

// TestValidationRegistration makes sure that any RESTStorage that allows create or update has the correct validation register.
// It doesn't guarantee that it's actually called, but it does guarantee that it at least exists
func TestValidationRegistration(t *testing.T) {
	config := fakeMasterConfig()

	storageMap := config.GetRestStorage()
	for key, resourceStorage := range storageMap {
		for resource, storage := range resourceStorage {
			obj := storage.New()
			kindType := reflect.TypeOf(obj)

			validationInfo, validatorExists := validation.Validator.GetInfo(obj)

			if _, ok := storage.(rest.Creater); ok {
				// if we're a creater, then we must have a validate method registered
				if !validatorExists {
					t.Errorf("No validator registered for %v (used by %s/%s).  Register in pkg/api/validation/register.go.", kindType, resource, key)
				}
			}

			if _, ok := storage.(rest.Updater); ok {
				exempted := false
				for _, t := range KnownUpdateValidationExceptions {
					if t == kindType {
						exempted = true
						break
					}
				}

				// if we're an updater, then we must have a validateUpdate method registered
				if !validatorExists && !exempted {
					t.Errorf("No validator registered for %v (used by %s/%s).  Register in pkg/api/validation/register.go.", kindType, resource, key)
				}

				if !validationInfo.UpdateAllowed && !exempted {
					t.Errorf("No validateUpdate method registered for %v (used by %s/%s).  Register in pkg/api/validation/register.go.", kindType, resource, key)
				}
			}
		}
	}
}

// fakeMasterConfig creates a new fake master config with an empty kubelet config and dummy storage.
func fakeMasterConfig() *MasterConfig {
	kubeInformerFactory := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 1*time.Second)
	informerFactory := shared.NewInformerFactory(kubeInformerFactory, fake.NewSimpleClientset(), testclient.NewSimpleFake(), shared.DefaultListerWatcherOverrides{}, 1*time.Second)
	return &MasterConfig{
		KubeletClientConfig:                   &kubeletclient.KubeletClientConfig{},
		RESTOptionsGetter:                     restoptions.NewSimpleGetter(&storagebackend.Config{ServerList: []string{"localhost"}}),
		Informers:                             informerFactory,
		ClusterQuotaMappingController:         clusterquotamapping.NewClusterQuotaMappingController(kubeInformerFactory.Namespaces(), informerFactory.ClusterResourceQuotas()),
		PrivilegedLoopbackKubernetesClientset: &kclientset.Clientset{},
	}
}
