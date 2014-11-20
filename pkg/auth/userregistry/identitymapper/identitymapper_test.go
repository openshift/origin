package identitymapper

import (
	"testing"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/registry/test"
)

func TestProvisionUser(t *testing.T) {
	userIdentityRegistry := &test.UserIdentityMappingRegistry{}
	providerId := "papa"
	identityMapper := NewAlwaysCreateUserIdentityToUserMapper(providerId, userIdentityRegistry)
	identity := &authapi.DefaultUserIdentityInfo{
		UserName: "oscar",
	}

	identityMapper.UserFor(identity)
	if userIdentityRegistry.CreatedUserIdentityMapping.Identity.Provider != providerId {
		t.Errorf("Expected provider to be set to %v, but it was actually %v", providerId, userIdentityRegistry.CreatedUserIdentityMapping.Identity.Provider)
	}
	if userIdentityRegistry.CreatedUserIdentityMapping.Identity.UserName != identity.GetUserName() {
		t.Errorf("Expected username to be set to %v, but it was actually %v", identity.GetUserName(), userIdentityRegistry.CreatedUserIdentityMapping.Identity.UserName)
	}

}
