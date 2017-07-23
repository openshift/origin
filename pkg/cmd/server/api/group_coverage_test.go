package api_test

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/server/api"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestKnownAPIGroups(t *testing.T) {
	unexposedGroups := sets.NewString("componentconfig", "metrics", "policy", "federation")

	enabledGroups := sets.NewString()
	for _, enabledVersion := range kapi.Registry.EnabledVersions() {
		enabledGroups.Insert(enabledVersion.Group)
	}

	// TODO remove this and use a non-global registry.  These are in a wierd half-state right now
	enabledGroups.Insert("apiextensions.k8s.io", "apiregistration.k8s.io")

	knownGroups := sets.NewString(api.KnownKubeAPIGroups.List()...)
	knownGroups.Insert(api.KnownOriginAPIGroups.List()...)

	if missingKnownGroups := knownGroups.Difference(enabledGroups); len(missingKnownGroups) > 0 {
		t.Errorf("KnownKubeAPIGroups or KnownOriginAPIGroups are missing from registered.EnabledVersions: %v", missingKnownGroups.List())
	}
	if unknownEnabledGroups := enabledGroups.Difference(knownGroups).Difference(unexposedGroups); len(unknownEnabledGroups) > 0 {
		t.Errorf("KnownKubeAPIGroups or KnownOriginAPIGroups is missing groups from registered.EnabledVersions: %v", unknownEnabledGroups.List())
	}
}

func TestAllowedAPIVersions(t *testing.T) {
	// TODO remove this and use a non-global registry.  These are in a wierd half-state right now
	skippedGroups := sets.NewString("apiextensions.k8s.io", "apiregistration.k8s.io")

	// Make sure all versions we know about match registered versions
	for group, versions := range api.KubeAPIGroupsToAllowedVersions {
		if skippedGroups.Has(group) {
			continue
		}

		enabled := sets.NewString()
		for _, enabledVersion := range kapi.Registry.EnabledVersionsForGroup(group) {
			enabled.Insert(enabledVersion.Version)
		}
		expected := sets.NewString(versions...)
		actual := enabled.Difference(sets.NewString(api.KubeDefaultDisabledVersions[group]...))
		if e, a := expected.List(), actual.List(); !reflect.DeepEqual(e, a) {
			t.Errorf("For group %s, expected versions %#v, got %#v", group, e, a)
		}
	}
}
