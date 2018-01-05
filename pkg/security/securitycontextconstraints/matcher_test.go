package securitycontextconstraints

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	allocator "github.com/openshift/origin/pkg/security"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"github.com/openshift/origin/pkg/security/uid"
)

func TestDeduplicateSecurityContextConstraints(t *testing.T) {
	duped := []*securityapi.SecurityContextConstraints{
		{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "c"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "d"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "e"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "e"}},
	}

	deduped := DeduplicateSecurityContextConstraints(duped)

	if len(deduped) != 5 {
		t.Fatalf("expected to have 5 remaining sccs but found %d: %v", len(deduped), deduped)
	}

	constraintCounts := map[string]int{}

	for _, scc := range deduped {
		if _, ok := constraintCounts[scc.Name]; !ok {
			constraintCounts[scc.Name] = 0
		}
		constraintCounts[scc.Name] = constraintCounts[scc.Name] + 1
	}

	for k, v := range constraintCounts {
		if v > 1 {
			t.Errorf("%s was found %d times after de-duping", k, v)
		}
	}

}

func TestAssignSecurityContext(t *testing.T) {
	// set up test data
	// scc that will deny privileged container requests and has a default value for a field (uid)
	var uid int64 = 9999
	fsGroup := int64(1)
	scc := &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test scc",
		},
		SELinuxContext: securityapi.SELinuxContextStrategyOptions{
			Type: securityapi.SELinuxStrategyRunAsAny,
		},
		RunAsUser: securityapi.RunAsUserStrategyOptions{
			Type: securityapi.RunAsUserStrategyMustRunAs,
			UID:  &uid,
		},

		// require allocation for a field in the psc as well to test changes/no changes
		FSGroup: securityapi.FSGroupStrategyOptions{
			Type: securityapi.FSGroupStrategyMustRunAs,
			Ranges: []securityapi.IDRange{
				{Min: fsGroup, Max: fsGroup},
			},
		},
		SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
			Type: securityapi.SupplementalGroupsStrategyRunAsAny,
		},
	}
	provider, err := NewSimpleProvider(scc)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	createContainer := func(priv bool) kapi.Container {
		return kapi.Container{
			SecurityContext: &kapi.SecurityContext{
				Privileged: &priv,
			},
		}
	}

	testCases := map[string]struct {
		pod            *kapi.Pod
		shouldValidate bool
		expectedUID    *int64
	}{
		"pod and container SC is not changed when invalid": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					SecurityContext: &kapi.PodSecurityContext{},
					Containers:      []kapi.Container{createContainer(true)},
				},
			},
			shouldValidate: false,
		},
		"must validate all containers": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					// good pod and bad pod
					SecurityContext: &kapi.PodSecurityContext{},
					Containers:      []kapi.Container{createContainer(false), createContainer(true)},
				},
			},
			shouldValidate: false,
		},
		"pod validates": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					SecurityContext: &kapi.PodSecurityContext{},
					Containers:      []kapi.Container{createContainer(false)},
				},
			},
			shouldValidate: true,
		},
	}

	for i := 0; i < 2; i++ {
		for k, v := range testCases {
			v.pod.Spec.Containers, v.pod.Spec.InitContainers = v.pod.Spec.InitContainers, v.pod.Spec.Containers

			errs := AssignSecurityContext(provider, v.pod, nil)
			if v.shouldValidate && len(errs) > 0 {
				t.Errorf("%s expected to validate but received errors %v", k, errs)
				continue
			}
			if !v.shouldValidate && len(errs) == 0 {
				t.Errorf("%s expected validation errors but received none", k)
				continue
			}
		}
	}
}

func TestRequiresPreAllocatedUIDRange(t *testing.T) {
	var uid int64 = 1

	testCases := map[string]struct {
		scc      *securityapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &securityapi.SecurityContextConstraints{
				RunAsUser: securityapi.RunAsUserStrategyOptions{
					Type: securityapi.RunAsUserStrategyMustRunAs,
				},
			},
		},
		"run as any": {
			scc: &securityapi.SecurityContextConstraints{
				RunAsUser: securityapi.RunAsUserStrategyOptions{
					Type: securityapi.RunAsUserStrategyRunAsAny,
				},
			},
		},
		"run as non-root": {
			scc: &securityapi.SecurityContextConstraints{
				RunAsUser: securityapi.RunAsUserStrategyOptions{
					Type: securityapi.RunAsUserStrategyMustRunAsNonRoot,
				},
			},
		},
		"run as range": {
			scc: &securityapi.SecurityContextConstraints{
				RunAsUser: securityapi.RunAsUserStrategyOptions{
					Type: securityapi.RunAsUserStrategyMustRunAsRange,
				},
			},
			requires: true,
		},
		"run as range with specified params": {
			scc: &securityapi.SecurityContextConstraints{
				RunAsUser: securityapi.RunAsUserStrategyOptions{
					Type:        securityapi.RunAsUserStrategyMustRunAsRange,
					UIDRangeMin: &uid,
					UIDRangeMax: &uid,
				},
			},
		},
	}

	for k, v := range testCases {
		result := requiresPreAllocatedUIDRange(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestRequiresPreAllocatedSELinuxLevel(t *testing.T) {
	testCases := map[string]struct {
		scc      *securityapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &securityapi.SecurityContextConstraints{
				SELinuxContext: securityapi.SELinuxContextStrategyOptions{
					Type: securityapi.SELinuxStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with level specified": {
			scc: &securityapi.SecurityContextConstraints{
				SELinuxContext: securityapi.SELinuxContextStrategyOptions{
					Type: securityapi.SELinuxStrategyMustRunAs,
					SELinuxOptions: &kapi.SELinuxOptions{
						Level: "foo",
					},
				},
			},
		},
		"run as any": {
			scc: &securityapi.SecurityContextConstraints{
				SELinuxContext: securityapi.SELinuxContextStrategyOptions{
					Type: securityapi.SELinuxStrategyRunAsAny,
				},
			},
		},
	}

	for k, v := range testCases {
		result := requiresPreAllocatedSELinuxLevel(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestRequiresPreallocatedSupplementalGroups(t *testing.T) {
	testCases := map[string]struct {
		scc      *securityapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &securityapi.SecurityContextConstraints{
				SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
					Type: securityapi.SupplementalGroupsStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with range specified": {
			scc: &securityapi.SecurityContextConstraints{
				SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
					Type: securityapi.SupplementalGroupsStrategyMustRunAs,
					Ranges: []securityapi.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		},
		"run as any": {
			scc: &securityapi.SecurityContextConstraints{
				SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
					Type: securityapi.SupplementalGroupsStrategyRunAsAny,
				},
			},
		},
	}
	for k, v := range testCases {
		result := requiresPreallocatedSupplementalGroups(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestRequiresPreallocatedFSGroup(t *testing.T) {
	testCases := map[string]struct {
		scc      *securityapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &securityapi.SecurityContextConstraints{
				FSGroup: securityapi.FSGroupStrategyOptions{
					Type: securityapi.FSGroupStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with range specified": {
			scc: &securityapi.SecurityContextConstraints{
				FSGroup: securityapi.FSGroupStrategyOptions{
					Type: securityapi.FSGroupStrategyMustRunAs,
					Ranges: []securityapi.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		},
		"run as any": {
			scc: &securityapi.SecurityContextConstraints{
				FSGroup: securityapi.FSGroupStrategyOptions{
					Type: securityapi.FSGroupStrategyRunAsAny,
				},
			},
		},
	}
	for k, v := range testCases {
		result := requiresPreallocatedFSGroup(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestParseSupplementalGroupAnnotation(t *testing.T) {
	tests := map[string]struct {
		groups     string
		expected   []uid.Block
		shouldFail bool
	}{
		"single block slash": {
			groups: "1/5",
			expected: []uid.Block{
				{Start: 1, End: 5},
			},
		},
		"single block dash": {
			groups: "1-5",
			expected: []uid.Block{
				{Start: 1, End: 5},
			},
		},
		"multiple blocks": {
			groups: "1/5,6/5,11/5",
			expected: []uid.Block{
				{Start: 1, End: 5},
				{Start: 6, End: 10},
				{Start: 11, End: 15},
			},
		},
		"dash format": {
			groups: "1-5,6-10,11-15",
			expected: []uid.Block{
				{Start: 1, End: 5},
				{Start: 6, End: 10},
				{Start: 11, End: 15},
			},
		},
		"no blocks": {
			groups:     "",
			shouldFail: true,
		},
	}
	for k, v := range tests {
		blocks, err := parseSupplementalGroupAnnotation(v.groups)

		if v.shouldFail && err == nil {
			t.Errorf("%s was expected to fail but received no error and blocks %v", k, blocks)
			continue
		}

		if !v.shouldFail && err != nil {
			t.Errorf("%s had an unexpected error %v", k, err)
			continue
		}

		if len(blocks) != len(v.expected) {
			t.Errorf("%s received unexpected number of blocks expected: %v, actual %v", k, v.expected, blocks)
		}

		for _, b := range v.expected {
			if !hasBlock(b, blocks) {
				t.Errorf("%s was missing block %v", k, b)
			}
		}
	}
}

func hasBlock(block uid.Block, blocks []uid.Block) bool {
	for _, b := range blocks {
		if b.Start == block.Start && b.End == block.End {
			return true
		}
	}
	return false
}

func TestGetPreallocatedFSGroup(t *testing.T) {
	ns := func() *kapi.Namespace {
		return &kapi.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
		}
	}

	fallbackNS := ns()
	fallbackNS.Annotations[allocator.UIDRangeAnnotation] = "1/5"

	emptyAnnotationNS := ns()
	emptyAnnotationNS.Annotations[allocator.SupplementalGroupsAnnotation] = ""

	badBlockNS := ns()
	badBlockNS.Annotations[allocator.SupplementalGroupsAnnotation] = "foo"

	goodNS := ns()
	goodNS.Annotations[allocator.SupplementalGroupsAnnotation] = "1/5"

	tests := map[string]struct {
		ns         *kapi.Namespace
		expected   []securityapi.IDRange
		shouldFail bool
	}{
		"fall back to uid if sup group doesn't exist": {
			ns: fallbackNS,
			expected: []securityapi.IDRange{
				{Min: 1, Max: 1},
			},
		},
		"no annotation": {
			ns:         ns(),
			shouldFail: true,
		},
		"empty annotation": {
			ns:         emptyAnnotationNS,
			shouldFail: true,
		},
		"bad block": {
			ns:         badBlockNS,
			shouldFail: true,
		},
		"good sup group annotation": {
			ns: goodNS,
			expected: []securityapi.IDRange{
				{Min: 1, Max: 1},
			},
		},
	}

	for k, v := range tests {
		ranges, err := getPreallocatedFSGroup(v.ns)
		if v.shouldFail && err == nil {
			t.Errorf("%s was expected to fail but received no error and ranges %v", k, ranges)
			continue
		}

		if !v.shouldFail && err != nil {
			t.Errorf("%s had an unexpected error %v", k, err)
			continue
		}

		if len(ranges) != len(v.expected) {
			t.Errorf("%s received unexpected number of ranges expected: %v, actual %v", k, v.expected, ranges)
		}

		for _, r := range v.expected {
			if !hasRange(r, ranges) {
				t.Errorf("%s was missing range %v", k, r)
			}
		}
	}
}

func TestGetPreallocatedSupplementalGroups(t *testing.T) {
	ns := func() *kapi.Namespace {
		return &kapi.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
		}
	}

	fallbackNS := ns()
	fallbackNS.Annotations[allocator.UIDRangeAnnotation] = "1/5"

	emptyAnnotationNS := ns()
	emptyAnnotationNS.Annotations[allocator.SupplementalGroupsAnnotation] = ""

	badBlockNS := ns()
	badBlockNS.Annotations[allocator.SupplementalGroupsAnnotation] = "foo"

	goodNS := ns()
	goodNS.Annotations[allocator.SupplementalGroupsAnnotation] = "1/5"

	tests := map[string]struct {
		ns         *kapi.Namespace
		expected   []securityapi.IDRange
		shouldFail bool
	}{
		"fall back to uid if sup group doesn't exist": {
			ns: fallbackNS,
			expected: []securityapi.IDRange{
				{Min: 1, Max: 5},
			},
		},
		"no annotation": {
			ns:         ns(),
			shouldFail: true,
		},
		"empty annotation": {
			ns:         emptyAnnotationNS,
			shouldFail: true,
		},
		"bad block": {
			ns:         badBlockNS,
			shouldFail: true,
		},
		"good sup group annotation": {
			ns: goodNS,
			expected: []securityapi.IDRange{
				{Min: 1, Max: 5},
			},
		},
	}

	for k, v := range tests {
		ranges, err := getPreallocatedSupplementalGroups(v.ns)
		if v.shouldFail && err == nil {
			t.Errorf("%s was expected to fail but received no error and ranges %v", k, ranges)
			continue
		}

		if !v.shouldFail && err != nil {
			t.Errorf("%s had an unexpected error %v", k, err)
			continue
		}

		if len(ranges) != len(v.expected) {
			t.Errorf("%s received unexpected number of ranges expected: %v, actual %v", k, v.expected, ranges)
		}

		for _, r := range v.expected {
			if !hasRange(r, ranges) {
				t.Errorf("%s was missing range %v", k, r)
			}
		}
	}
}

func hasRange(rng securityapi.IDRange, ranges []securityapi.IDRange) bool {
	for _, r := range ranges {
		if r.Min == rng.Min && r.Max == rng.Max {
			return true
		}
	}
	return false
}
