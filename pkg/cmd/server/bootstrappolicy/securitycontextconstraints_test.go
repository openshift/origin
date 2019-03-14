package bootstrappolicy

import (
	"sort"
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	sccutil "github.com/openshift/origin/pkg/security/securitycontextconstraints/util"
	sccsort "github.com/openshift/origin/pkg/security/securitycontextconstraints/util/sort"
)

func TestBootstrappedConstraints(t *testing.T) {
	// ordering of expectedConstraintNames is important, we check it against scc.ByPriority
	expectedConstraintNames := []string{
		SecurityContextConstraintsAnyUID,
		SecurityContextConstraintRestricted,
		SecurityContextConstraintNonRoot,
		SecurityContextConstraintHostMountAndAnyUID,
		SecurityContextConstraintsHostNetwork,
		SecurityContextConstraintHostNS,
		SecurityContextConstraintPrivileged,
	}
	expectedVolumes := []securityv1.FSType{securityv1.FSTypeEmptyDir, securityv1.FSTypeSecret, securityv1.FSTypeDownwardAPI, securityv1.FSTypeConfigMap, securityv1.FSTypePersistentVolumeClaim}

	bootstrappedConstraints := GetBootstrapSecurityContextConstraints()

	if len(expectedConstraintNames) != len(bootstrappedConstraints) {
		t.Errorf("unexpected number of constraints: found %d, wanted %d", len(bootstrappedConstraints), len(expectedConstraintNames))
	}

	bootstrappedConstraintsExternal, err := sccsort.ByPriorityConvert(bootstrappedConstraints)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Sort(bootstrappedConstraintsExternal)

	for i, constraint := range bootstrappedConstraintsExternal {
		if constraint.Name != expectedConstraintNames[i] {
			t.Errorf("unexpected contraint no. %d (by priority).  Found %v, wanted %v", i, constraint.Name, expectedConstraintNames[i])
		}

		for _, expectedVolume := range expectedVolumes {
			if !sccutil.SCCAllowsFSType(constraint, expectedVolume) {
				t.Errorf("%s does not support %v which is required for all default SCCs", constraint.Name, expectedVolume)
			}
		}
	}
}
