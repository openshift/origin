package bootstrappolicy

import (
	"reflect"
	"testing"
)

func TestBootstrappedConstraints(t *testing.T) {
	expectedConstraints := []string{
		SecurityContextConstraintPrivileged,
		SecurityContextConstraintRestricted,
		SecurityContextConstraintNonRoot,
		SecurityContextConstraintHostMount,
		SecurityContextConstraintHostNS,
		SecurityContextConstraintsAnyUID,
	}
	expectedGroups, expectedUsers := getExpectedAccess()

	groups, users := GetBoostrapSCCAccess()
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
	}
}

func TestBootstrappedConstraintsWithAddedUser(t *testing.T) {
	expectedGroups, expectedUsers := getExpectedAccess()

	// get default access and add our own user to it
	groups, users := GetBoostrapSCCAccess()
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
	users := map[string][]string{}
	return groups, users
}
