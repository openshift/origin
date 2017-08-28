package bootstrappolicy

import (
	"reflect"
	"testing"

	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	sccutil "github.com/openshift/origin/pkg/security/securitycontextconstraints/util"
)

func TestBootstrappedConstraints(t *testing.T) {
	expectedConstraints := []string{
		SecurityContextConstraintPrivileged,
		SecurityContextConstraintRestricted,
		SecurityContextConstraintNonRoot,
		SecurityContextConstraintHostMountAndAnyUID,
		SecurityContextConstraintHostNS,
		SecurityContextConstraintsAnyUID,
		SecurityContextConstraintsHostNetwork,
	}
	expectedGroups, expectedUsers := getExpectedAccess()
	expectedVolumes := []securityapi.FSType{securityapi.FSTypeEmptyDir, securityapi.FSTypeSecret, securityapi.FSTypeDownwardAPI, securityapi.FSTypeConfigMap, securityapi.FSTypePersistentVolumeClaim}

	groups, users := GetBoostrapSCCAccess(DefaultOpenShiftInfraNamespace)
	bootstrappedConstraints := GetBootstrapSecurityContextConstraints(groups, users)

	if len(expectedConstraints) != len(bootstrappedConstraints) {
		t.Errorf("unexpected number of constraints: found %d, wanted %d", len(bootstrappedConstraints), len(expectedConstraints))
	}

	for _, constraint := range bootstrappedConstraints {
		g := expectedGroups[constraint.Name]
		if !reflect.DeepEqual(g, constraint.Groups) {
			t.Errorf("unexpected group access for %s.  Found %v, wanted %v", constraint.Name, constraint.Groups, g)
		}

		u := expectedUsers[constraint.Name]
		if !reflect.DeepEqual(u, constraint.Users) {
			t.Errorf("unexpected user access for %s.  Found %v, wanted %v", constraint.Name, constraint.Users, u)
		}

		for _, expectedVolume := range expectedVolumes {
			if !sccutil.SCCAllowsFSType(constraint, expectedVolume) {
				t.Errorf("%s does not support %v which is required for all default SCCs", constraint.Name, expectedVolume)
			}
		}
	}
}

func TestBootstrappedConstraintsWithAddedUser(t *testing.T) {
	expectedGroups, expectedUsers := getExpectedAccess()

	// get default access and add our own user to it
	groups, users := GetBoostrapSCCAccess(DefaultOpenShiftInfraNamespace)
	users[SecurityContextConstraintPrivileged] = append(users[SecurityContextConstraintPrivileged], "foo")
	bootstrappedConstraints := GetBootstrapSecurityContextConstraints(groups, users)

	// add it to expected
	expectedUsers[SecurityContextConstraintPrivileged] = append(expectedUsers[SecurityContextConstraintPrivileged], "foo")

	for _, constraint := range bootstrappedConstraints {
		g := expectedGroups[constraint.Name]
		if !reflect.DeepEqual(g, constraint.Groups) {
			t.Errorf("unexpected group access for %s.  Found %v, wanted %v", constraint.Name, constraint.Groups, g)
		}

		u := expectedUsers[constraint.Name]
		if !reflect.DeepEqual(u, constraint.Users) {
			t.Errorf("unexpected user access for %s.  Found %v, wanted %v", constraint.Name, constraint.Users, u)
		}
	}
}

func getExpectedAccess() (map[string][]string, map[string][]string) {
	groups := map[string][]string{
		SecurityContextConstraintPrivileged: {ClusterAdminGroup, NodesGroup},
		SecurityContextConstraintsAnyUID:    {ClusterAdminGroup},
		SecurityContextConstraintRestricted: {AuthenticatedGroup},
	}

	buildControllerUsername := serviceaccount.MakeUsername(DefaultOpenShiftInfraNamespace, InfraBuildControllerServiceAccountName)
	pvRecyclerControllerUsername := serviceaccount.MakeUsername(DefaultOpenShiftInfraNamespace, InfraPersistentVolumeRecyclerControllerServiceAccountName)
	users := map[string][]string{
		SecurityContextConstraintPrivileged:         {buildControllerUsername},
		SecurityContextConstraintHostMountAndAnyUID: {pvRecyclerControllerUsername},
	}
	return groups, users
}
