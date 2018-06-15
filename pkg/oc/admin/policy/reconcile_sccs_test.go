package policy

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestComputeDefinitions(t *testing.T) {
	diffPriv := goodSCC()
	diffPriv.AllowPrivilegedContainer = true

	diffCaps := goodSCC()
	diffCaps.AllowedCapabilities = []kapi.Capability{"foo"}

	diffHostDir := goodSCC()
	diffHostDir.Volumes = []securityapi.FSType{securityapi.FSTypeHostPath}

	diffHostNetwork := goodSCC()
	diffHostNetwork.AllowHostNetwork = true

	diffHostPorts := goodSCC()
	diffHostPorts.AllowHostPorts = true

	diffHostPID := goodSCC()
	diffHostPID.AllowHostPID = true

	diffHostIPC := goodSCC()
	diffHostIPC.AllowHostIPC = true

	diffSELinux := goodSCC()
	diffSELinux.SELinuxContext.Type = securityapi.SELinuxStrategyMustRunAs

	diffRunAsUser := goodSCC()
	diffRunAsUser.RunAsUser.Type = securityapi.RunAsUserStrategyMustRunAs

	diffSupGroups := goodSCC()
	diffSupGroups.SupplementalGroups.Type = securityapi.SupplementalGroupsStrategyMustRunAs

	diffFSGroup := goodSCC()
	diffFSGroup.FSGroup.Type = securityapi.FSGroupStrategyMustRunAs

	diffVolumes := goodSCC()
	diffVolumes.Volumes = []securityapi.FSType{securityapi.FSTypeAWSElasticBlockStore}

	noDiffVolumesA := goodSCC()
	noDiffVolumesA.Volumes = []securityapi.FSType{securityapi.FSTypeAWSElasticBlockStore, securityapi.FSTypeHostPath}
	noDiffVolumesB := goodSCC()
	noDiffVolumesB.Volumes = []securityapi.FSType{securityapi.FSTypeHostPath, securityapi.FSTypeAWSElasticBlockStore}

	tests := map[string]struct {
		expected    securityapi.SecurityContextConstraints
		actual      securityapi.SecurityContextConstraints
		needsUpdate bool
	}{
		"different priv": {
			expected:    goodSCC(),
			actual:      diffPriv,
			needsUpdate: true,
		},
		"different caps": {
			expected:    goodSCC(),
			actual:      diffCaps,
			needsUpdate: true,
		},
		"different host dir": {
			expected:    goodSCC(),
			actual:      diffHostDir,
			needsUpdate: true,
		},
		"different host network": {
			expected:    goodSCC(),
			actual:      diffHostNetwork,
			needsUpdate: true,
		},
		"different host ports": {
			expected:    goodSCC(),
			actual:      diffHostPorts,
			needsUpdate: true,
		},
		"different host pid": {
			expected:    goodSCC(),
			actual:      diffHostPID,
			needsUpdate: true,
		},
		"different host IPC": {
			expected:    goodSCC(),
			actual:      diffHostIPC,
			needsUpdate: true,
		},
		"different host SELinux": {
			expected:    goodSCC(),
			actual:      diffSELinux,
			needsUpdate: true,
		},
		"different host RunAsUser": {
			expected:    goodSCC(),
			actual:      diffRunAsUser,
			needsUpdate: true,
		},
		"different host Sup Groups": {
			expected:    goodSCC(),
			actual:      diffSupGroups,
			needsUpdate: true,
		},
		"different host FS Group": {
			expected:    goodSCC(),
			actual:      diffFSGroup,
			needsUpdate: true,
		},
		"different volumes": {
			expected:    goodSCC(),
			actual:      diffVolumes,
			needsUpdate: true,
		},
		"unsorted volumes": {
			expected:    noDiffVolumesA,
			actual:      noDiffVolumesB,
			needsUpdate: false,
		},
		"no diff": {
			expected:    goodSCC(),
			actual:      goodSCC(),
			needsUpdate: false,
		},
	}

	for k, v := range tests {
		cmd := NewDefaultReconcileSCCOptions()

		computedSCC, needsUpdate := cmd.computeUpdatedSCC(v.expected, v.actual)
		// ensure we got an update
		if needsUpdate != v.needsUpdate {
			t.Errorf("%s expected to need an update but did not trigger one", k)
		}

		if !reflect.DeepEqual(&v.expected, computedSCC) {
			t.Errorf("unexpected diffs were produced from %s", k)
			t.Logf("wanted: %v", &v.expected)
			t.Logf("got:    %v", computedSCC)
		}

		// ensure that if the case needed an update that no diff results from passing through again
		if v.needsUpdate {
			if _, doubleUpdate := cmd.computeUpdatedSCC(v.expected, *computedSCC); doubleUpdate {
				t.Errorf("%s resulted in an SCC that needed update even after computing", k)
			}
		}
	}
}

func TestComputeMetadata(t *testing.T) {
	tests := map[string]struct {
		union       bool
		desired     metav1.ObjectMeta
		actual      metav1.ObjectMeta
		needsUpdate bool
		computed    metav1.ObjectMeta
	}{
		"identical with union": {
			union: true,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a"},
				Annotations: map[string]string{"annotationa": "a"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
			needsUpdate: false,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
		},
		"identical without union": {
			union: false,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a"},
				Annotations: map[string]string{"annotationa": "a"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
			needsUpdate: false,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
		},

		"missing labels and annotations with union": {
			union: true,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a", "labelb": "b"},
				Annotations: map[string]string{"annotationa": "a", "annotationb": "b"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
			needsUpdate: true,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a", "labelb": "b"},
				Annotations:     map[string]string{"annotationa": "a", "annotationb": "b"},
				ResourceVersion: "2",
			},
		},
		"missing labels and annotations without union": {
			union: false,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a", "labelb": "b"},
				Annotations: map[string]string{"annotationa": "a", "annotationb": "b"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
			needsUpdate: true,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a", "labelb": "b"},
				Annotations:     map[string]string{"annotationa": "a", "annotationb": "b"},
				ResourceVersion: "2",
			},
		},

		"extra labels and annotations with union": {
			union: true,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a"},
				Annotations: map[string]string{"annotationa": "a"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a", "labelb": "b"},
				Annotations:     map[string]string{"annotationa": "a", "annotationb": "b"},
				ResourceVersion: "2",
			},
			needsUpdate: false,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a", "labelb": "b"},
				Annotations:     map[string]string{"annotationa": "a", "annotationb": "b"},
				ResourceVersion: "2",
			},
		},
		"extra labels and annotations without union": {
			union: false,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a"},
				Annotations: map[string]string{"annotationa": "a"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a", "labelb": "b"},
				Annotations:     map[string]string{"annotationa": "a", "annotationb": "b"},
				ResourceVersion: "2",
			},
			needsUpdate: true,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
		},

		"disjoint labels and annotations with union": {
			union: true,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a"},
				Annotations: map[string]string{"annotationa": "a"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labelb": "b"},
				Annotations:     map[string]string{"annotationb": "b"},
				ResourceVersion: "2",
			},
			needsUpdate: true,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a", "labelb": "b"},
				Annotations:     map[string]string{"annotationa": "a", "annotationb": "b"},
				ResourceVersion: "2",
			},
		},
		"disjoint labels and annotations without union": {
			union: false,
			desired: metav1.ObjectMeta{
				Name:        "foo",
				Labels:      map[string]string{"labela": "a"},
				Annotations: map[string]string{"annotationa": "a"},
			},
			actual: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labelb": "b"},
				Annotations:     map[string]string{"annotationb": "b"},
				ResourceVersion: "2",
			},
			needsUpdate: true,
			computed: metav1.ObjectMeta{
				Name:            "foo",
				Labels:          map[string]string{"labela": "a"},
				Annotations:     map[string]string{"annotationa": "a"},
				ResourceVersion: "2",
			},
		},
	}

	for k, v := range tests {
		cmd := NewDefaultReconcileSCCOptions()
		cmd.Union = v.union

		desiredSCC := goodSCC()
		desiredSCC.ObjectMeta = v.desired

		actualSCC := goodSCC()
		actualSCC.ObjectMeta = v.actual

		computedSCC, needsUpdate := cmd.computeUpdatedSCC(desiredSCC, actualSCC)
		if needsUpdate != v.needsUpdate {
			t.Errorf("%s expected needsUpdate=%v, got %v", k, v.needsUpdate, needsUpdate)
			continue
		}
		if !reflect.DeepEqual(v.computed, computedSCC.ObjectMeta) {
			t.Errorf("%s: expected object meta\n%#v\ngot\n%#v", k, v.computed, computedSCC.ObjectMeta)
			continue
		}
	}
}

func TestComputeUnioningUsersAndGroups(t *testing.T) {
	missingGroup := goodSCC()
	missingGroup.Groups = []string{"foo"}

	missingUser := goodSCC()
	missingUser.Users = []string{"foo"}

	tests := map[string]struct {
		expected       securityapi.SecurityContextConstraints
		actual         securityapi.SecurityContextConstraints
		expectedGroups []string
		expectedUsers  []string
		needsUpdate    bool
		union          bool
	}{
		"missing group grants": {
			expected:       goodSCC(),
			actual:         missingGroup,
			expectedGroups: append(missingGroup.Groups, goodSCC().Groups...),
			expectedUsers:  goodSCC().Users,
			needsUpdate:    true,
			union:          true,
		},
		"missing user grants": {
			expected:       goodSCC(),
			actual:         missingUser,
			expectedGroups: goodSCC().Groups,
			expectedUsers:  append(missingUser.Users, goodSCC().Users...),
			needsUpdate:    true,
			union:          true,
		},
		"no diff union": {
			expected:       goodSCC(),
			actual:         goodSCC(),
			expectedGroups: goodSCC().Groups,
			expectedUsers:  goodSCC().Users,
			needsUpdate:    false,
			union:          true,
		},
		"non-unioning user": {
			// non-union tc will use the values in expected to compare, no need to set here
			expected:    goodSCC(),
			actual:      missingUser,
			needsUpdate: true,
			union:       false,
		},
		"non-unioning group": {
			// non-union tc will use the values in expected to compare, no need to set here
			expected:    goodSCC(),
			actual:      missingGroup,
			needsUpdate: true,
			union:       false,
		},
		"no diff non-union": {
			// non-union tc will use the values in expected to compare, no need to set here
			expected:    goodSCC(),
			actual:      goodSCC(),
			needsUpdate: false,
			union:       false,
		},
	}

	for k, v := range tests {
		cmd := NewDefaultReconcileSCCOptions()
		cmd.Union = v.union

		computedSCC, needsUpdate := cmd.computeUpdatedSCC(v.expected, v.actual)
		// ensure we got an update
		if needsUpdate != v.needsUpdate {
			t.Errorf("%s expected to need an update but did not trigger one", k)
		}

		toCompareGroups := v.expectedGroups
		toCompareUsers := v.expectedUsers
		// if not unioning then we should be reset to the expected groups/users
		if !v.union {
			toCompareGroups = v.expected.Groups
			toCompareUsers = v.expected.Users
		}
		// ensure that we ended up with the union we expected
		if !reflect.DeepEqual(computedSCC.Groups, toCompareGroups) {
			t.Errorf("%s had unexpected groups wanted: %v, got: %v", k, toCompareGroups, computedSCC.Groups)
		}
		if !reflect.DeepEqual(computedSCC.Users, toCompareUsers) {
			t.Errorf("%s had unexpected users wanted: %v, got: %v", k, toCompareUsers, computedSCC.Users)
		}

		// ensure the computed scc doesn't trigger additional updates
		if v.needsUpdate {
			if _, doubleUpdate := cmd.computeUpdatedSCC(v.expected, *computedSCC); doubleUpdate {
				t.Errorf("%s resulted in an SCC that needed update even after computing", k)
			}
		}
	}
}

func TestComputeUnioningPriorities(t *testing.T) {
	priorityOne := int32(1)
	priorityTwo := int32(2)

	tests := map[string]struct {
		expected         securityapi.SecurityContextConstraints
		actual           securityapi.SecurityContextConstraints
		expectedPriority *int32
		needsUpdate      bool
		union            bool
	}{
		"not overwriting priorities, nil actual and non-nil expected": {
			expected:         goodSCCWithPriority(priorityOne),
			actual:           goodSCC(),
			expectedPriority: &priorityOne,
			needsUpdate:      true,
			union:            true,
		},
		"not overwriting priorities, non-nil actual and non-nil expected": {
			expected:         goodSCCWithPriority(priorityOne),
			actual:           goodSCCWithPriority(priorityTwo),
			expectedPriority: &priorityTwo,
			needsUpdate:      false,
			union:            true,
		},
		"not overwriting priorities, non-nil actual and nil expected": {
			expected:         goodSCC(),
			actual:           goodSCCWithPriority(priorityTwo),
			expectedPriority: &priorityTwo,
			needsUpdate:      false,
			union:            true,
		},
		"not overwriting priorities, both nil": {
			expected:         goodSCC(),
			actual:           goodSCC(),
			expectedPriority: nil,
			needsUpdate:      false,
			union:            true,
		},
		"not overwriting priorities, no diff": {
			expected:         goodSCCWithPriority(priorityOne),
			actual:           goodSCCWithPriority(priorityOne),
			expectedPriority: &priorityOne,
			needsUpdate:      false,
			union:            true,
		},
		"overwriting priorities, nil actual and non-nil expected": {
			expected:         goodSCCWithPriority(priorityOne),
			actual:           goodSCC(),
			expectedPriority: &priorityOne,
			needsUpdate:      true,
			union:            false,
		},
		"overwriting priorities, non-nil actual and non-nil expected": {
			expected:         goodSCCWithPriority(priorityOne),
			actual:           goodSCCWithPriority(priorityTwo),
			expectedPriority: &priorityOne,
			needsUpdate:      true,
			union:            false,
		},
		"overwriting priorities, nil actual and nil expected": {
			expected:         goodSCC(),
			actual:           goodSCC(),
			expectedPriority: nil,
			needsUpdate:      false,
			union:            false,
		},
		"overwriting priorities, non-nil actual and nil expected": {
			expected:         goodSCC(),
			actual:           goodSCCWithPriority(priorityTwo),
			expectedPriority: nil,
			needsUpdate:      true,
			union:            false,
		},
		"overwriting priorities, no diff": {
			expected:         goodSCCWithPriority(priorityTwo),
			actual:           goodSCCWithPriority(priorityTwo),
			expectedPriority: &priorityTwo,
			needsUpdate:      false,
			union:            false,
		},
	}

	for k, v := range tests {
		cmd := NewDefaultReconcileSCCOptions()
		cmd.Union = v.union

		computedSCC, needsUpdate := cmd.computeUpdatedSCC(v.expected, v.actual)
		// ensure we got an update
		if needsUpdate != v.needsUpdate {
			t.Errorf("%s expected to need an update but did not trigger one", k)
		}

		// ensure priorities are set correctly
		if v.expectedPriority != nil && computedSCC.Priority == nil {
			t.Errorf("%s expected a non nil computed priority", k)
		}
		if v.expectedPriority == nil && computedSCC.Priority != nil {
			t.Errorf("%s expected a nil priority but got %d", k, *computedSCC.Priority)
		}
		if v.expectedPriority != nil && computedSCC.Priority != nil && *v.expectedPriority != *computedSCC.Priority {
			t.Errorf("%s expected priority %d but got %d", k, *v.expectedPriority, *computedSCC.Priority)
		}

		// ensure the computed scc doesn't trigger additional updates
		if v.needsUpdate {
			if _, doubleUpdate := cmd.computeUpdatedSCC(v.expected, *computedSCC); doubleUpdate {
				t.Errorf("%s resulted in an SCC that needed update even after computing", k)
			}
		}
	}
}

func goodSCCWithPriority(priority int32) securityapi.SecurityContextConstraints {
	scc := goodSCC()
	scc.Priority = &priority
	return scc
}

func goodSCC() securityapi.SecurityContextConstraints {
	return securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-admin",
		},
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyRunAsAny,
		},
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyRunAsAny,
		},
		Users:  []string{"user"},
		Groups: []string{"group"},
	}
}
