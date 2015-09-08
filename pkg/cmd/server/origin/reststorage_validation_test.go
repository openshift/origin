package origin

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api/rest"
	kclient "k8s.io/kubernetes/pkg/client"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/validation"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// TestValidationRegistration makes sure that any RESTStorage that allows create or update has the correct validation register.
// It doesn't guarantee that it's actually called, but it does guarantee that it at least exists
func TestValidationRegistration(t *testing.T) {
	config := fakeMasterConfig()

	storageMap := config.GetRestStorage()
	for key, storage := range storageMap {
		obj := storage.New()
		kindType := reflect.TypeOf(obj)

		validationInfo, validatorExists := validation.Validator.GetInfo(obj)

		if _, ok := storage.(rest.Creater); ok {
			// if we're a creater, then we must have a validate method registered
			if !validatorExists {
				t.Errorf("No validator registered for %v (used by %v).  Register in pkg/api/validation/register.go.", kindType, key)
			}
		}

		if _, ok := storage.(rest.Updater); ok {
			// if we're an updater, then we must have a validateUpdate method registered
			if !validatorExists {
				t.Errorf("No validator registered for %v (used by %v).  Register in pkg/api/validation/register.go.", kindType, key)
			}

			if !validationInfo.UpdateAllowed {
				t.Errorf("No validateUpdate method registered for %v (used by %v).  Register in pkg/api/validation/register.go.", kindType, key)
			}
		}

	}
}

// TestAllOpenShiftResourceCoverage checks to make sure that the openshift all group actually contains all openshift resources
func TestAllOpenShiftResourceCoverage(t *testing.T) {
	allOpenshift := authorizationapi.ExpandResources(util.NewStringSet(authorizationapi.GroupsToResources[authorizationapi.OpenshiftAllGroupName]...))

	config := fakeMasterConfig()

	storageMap := config.GetRestStorage()
	for key := range storageMap {
		if allOpenshift.Has(strings.ToLower(key)) {
			continue
		}

		t.Errorf("authorizationapi.GroupsToResources[authorizationapi.OpenshiftAllGroupName] is missing %v.  Check pkg/authorization/api/types.go.", strings.ToLower(key))
	}
}

// fakeMasterConfig creates a new fake master config with an empty kubelet config and dummy storage.
func fakeMasterConfig() *MasterConfig {
	return &MasterConfig{
		KubeletClientConfig: &kclient.KubeletConfig{},
		EtcdHelper:          etcdstorage.NewEtcdStorage(nil, nil, ""),
	}
}
