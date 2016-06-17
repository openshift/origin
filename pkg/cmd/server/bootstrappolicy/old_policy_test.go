package bootstrappolicy_test

import (
	"testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// leave this in place so I can use when converting the SAs
func DisableTestClusterRoles(t *testing.T) {
	currentRoles := bootstrappolicy.GetBootstrapClusterRoles()
	oldRoles := oldGetBootstrapClusterRoles()

	// old roles don't have the SAs appended, so run through them.  The SAs haven't been converted yet
	for i := range oldRoles {
		oldRole := oldRoles[i]
		newRole := currentRoles[i]

		if oldRole.Name != newRole.Name {
			t.Fatalf("%v vs %v", oldRole.Name, newRole.Name)
		}

		// @liggitt don't whine about a temporary test fataling
		if covers, missing := rulevalidation.Covers(oldRole.Rules, newRole.Rules); !covers {
			t.Fatalf("%v/%v: %#v", oldRole.Name, newRole.Name, missing)
		}
		if covers, missing := rulevalidation.Covers(newRole.Rules, oldRole.Rules); !covers {
			t.Fatalf("%v/%v: %#v", oldRole.Name, newRole.Name, missing)
		}
	}
}

func oldGetBootstrapClusterRoles() []authorizationapi.ClusterRole {
	return nil
}
