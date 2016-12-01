package scc

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/conversion"
	kscc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/util/diff"

	allocator "github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
)

func TestDeduplicateSecurityContextConstraints(t *testing.T) {
	duped := []*kapi.SecurityContextConstraints{
		{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "c"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "d"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "e"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "e"}},
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
	scc := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test scc",
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &uid,
		},

		// require allocation for a field in the psc as well to test changes/no changes
		FSGroup: kapi.FSGroupStrategyOptions{
			Type: kapi.FSGroupStrategyMustRunAs,
			Ranges: []kapi.IDRange{
				{Min: fsGroup, Max: fsGroup},
			},
		},
		SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
			Type: kapi.SupplementalGroupsStrategyRunAsAny,
		},
	}
	provider, err := kscc.NewSimpleProvider(scc)
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

	// these are set up such that the containers always have a nil uid.  If the case should not
	// validate then the uids should not have been updated by the strategy.  If the case should
	// validate then uids should be set.  This is ensuring that we're hanging on to the old SC
	// as we generate/validate and only updating the original container if the entire pod validates
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

			clonedPod, err := conversion.NewCloner().DeepCopy(v.pod)
			if err != nil {
				t.Errorf("Unable to clone pod: %v", err)
				continue
			}
			psc, annotations, cscs, errs := ResolvePodSecurityContext(provider, v.pod)

			//check whether ResolvePodSecurityContext mutate the provided pod
			if !kapi.Semantic.DeepEqual(v.pod, clonedPod) {
				t.Errorf("%s expected immutated pod %s", k, diff.ObjectDiff(v.pod, clonedPod))
				continue
			}

			if v.shouldValidate && len(errs) > 0 {
				t.Errorf("%s expected to validate but received errors %v", k, errs)
				continue
			}
			if !v.shouldValidate && len(errs) == 0 {
				t.Errorf("%s expected validation errors but received none", k)
				continue
			}

			if len(errs) == 0 {
				SetSecurityContext(v.pod, psc, annotations, cscs)
			}

			// if we shouldn't have validated ensure that uid is not set on the containers
			// and ensure the psc does not have fsgroup set
			if !v.shouldValidate {
				if v.pod.Spec.SecurityContext.FSGroup != nil {
					t.Errorf("%s had a non-nil FSGroup %d.  FSGroup should not be set on test cases that don't validate", k, *v.pod.Spec.SecurityContext.FSGroup)
				}
				for _, c := range v.pod.Spec.Containers {
					if c.SecurityContext.RunAsUser != nil {
						t.Errorf("%s had non-nil UID %d.  UID should not be set on test cases that don't validate", k, *c.SecurityContext.RunAsUser)
					}
				}
			}

			// if we validated then the pod sc should be updated now with the defaults from the SCC
			if v.shouldValidate {
				if *v.pod.Spec.SecurityContext.FSGroup != fsGroup {
					t.Errorf("%s expected fsgroup to be defaulted but found %v", k, v.pod.Spec.SecurityContext.FSGroup)
				}
				for _, c := range v.pod.Spec.Containers {
					if *c.SecurityContext.RunAsUser != uid {
						t.Errorf("%s expected uid to be defaulted to %d but found %v", k, uid, c.SecurityContext.RunAsUser)
					}
				}
			}
		}
	}
}

func TestRequiresPreAllocatedUIDRange(t *testing.T) {
	var uid int64 = 1

	testCases := map[string]struct {
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyMustRunAs,
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyRunAsAny,
				},
			},
		},
		"run as non-root": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyMustRunAsNonRoot,
				},
			},
		},
		"run as range": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type: kapi.RunAsUserStrategyMustRunAsRange,
				},
			},
			requires: true,
		},
		"run as range with specified params": {
			scc: &kapi.SecurityContextConstraints{
				RunAsUser: kapi.RunAsUserStrategyOptions{
					Type:        kapi.RunAsUserStrategyMustRunAsRange,
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
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				SELinuxContext: kapi.SELinuxContextStrategyOptions{
					Type: kapi.SELinuxStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with level specified": {
			scc: &kapi.SecurityContextConstraints{
				SELinuxContext: kapi.SELinuxContextStrategyOptions{
					Type: kapi.SELinuxStrategyMustRunAs,
					SELinuxOptions: &kapi.SELinuxOptions{
						Level: "foo",
					},
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				SELinuxContext: kapi.SELinuxContextStrategyOptions{
					Type: kapi.SELinuxStrategyRunAsAny,
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
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
					Type: kapi.SupplementalGroupsStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with range specified": {
			scc: &kapi.SecurityContextConstraints{
				SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
					Type: kapi.SupplementalGroupsStrategyMustRunAs,
					Ranges: []kapi.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
					Type: kapi.SupplementalGroupsStrategyRunAsAny,
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
		scc      *kapi.SecurityContextConstraints
		requires bool
	}{
		"must run as": {
			scc: &kapi.SecurityContextConstraints{
				FSGroup: kapi.FSGroupStrategyOptions{
					Type: kapi.FSGroupStrategyMustRunAs,
				},
			},
			requires: true,
		},
		"must with range specified": {
			scc: &kapi.SecurityContextConstraints{
				FSGroup: kapi.FSGroupStrategyOptions{
					Type: kapi.FSGroupStrategyMustRunAs,
					Ranges: []kapi.IDRange{
						{Min: 1, Max: 1},
					},
				},
			},
		},
		"run as any": {
			scc: &kapi.SecurityContextConstraints{
				FSGroup: kapi.FSGroupStrategyOptions{
					Type: kapi.FSGroupStrategyRunAsAny,
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
			ObjectMeta: kapi.ObjectMeta{
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
		expected   []kapi.IDRange
		shouldFail bool
	}{
		"fall back to uid if sup group doesn't exist": {
			ns: fallbackNS,
			expected: []kapi.IDRange{
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
			expected: []kapi.IDRange{
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
			ObjectMeta: kapi.ObjectMeta{
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
		expected   []kapi.IDRange
		shouldFail bool
	}{
		"fall back to uid if sup group doesn't exist": {
			ns: fallbackNS,
			expected: []kapi.IDRange{
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
			expected: []kapi.IDRange{
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

func hasRange(rng kapi.IDRange, ranges []kapi.IDRange) bool {
	for _, r := range ranges {
		if r.Min == rng.Min && r.Max == rng.Max {
			return true
		}
	}
	return false
}

func TestCreateProviderFromConstraint(t *testing.T) {
	namespaceValid := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoUID := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}
	namespaceNoMCS := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.SupplementalGroupsAnnotation: "1/3",
			},
		},
	}

	namespaceNoSupplementalGroupsFallbackToUID := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
				allocator.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}

	namespaceBadSupGroups := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "",
			},
		},
	}

	testCases := map[string]struct {
		// use a generating function so we can test for non-mutation
		scc         func() *kapi.SecurityContextConstraints
		namespace   *kapi.Namespace
		expectedErr string
	}{
		"valid non-preallocated scc": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "valid non-preallocated scc",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace: namespaceValid,
		},
		"valid pre-allocated scc": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "valid pre-allocated scc",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type:           kapi.SELinuxStrategyMustRunAs,
						SELinuxOptions: &kapi.SELinuxOptions{User: "myuser"},
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceValid,
		},
		"pre-allocated no uid annotation": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no uid annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyMustRunAs,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoUID,
			expectedErr: "unable to find pre-allocated uid annotation",
		},
		"pre-allocated no mcs annotation": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no mcs annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyMustRunAs,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceNoMCS,
			expectedErr: "unable to find pre-allocated mcs annotation",
		},
		"pre-allocated group falls back to UID annotation": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace: namespaceNoSupplementalGroupsFallbackToUID,
		},
		"pre-allocated group bad value fails": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no sup group annotation",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyMustRunAs,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyMustRunAs,
					},
				}
			},
			namespace:   namespaceBadSupGroups,
			expectedErr: "unable to find pre-allocated group annotation",
		},
		"bad scc strategy options": {
			scc: func() *kapi.SecurityContextConstraints {
				return &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{
						Name: "bad scc user options",
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAs,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
				}
			},
			namespace:   namespaceValid,
			expectedErr: "MustRunAs requires a UID",
		},
	}

	for k, v := range testCases {
		scc := v.scc()
		_, err := createProviderFromConstraint(v.namespace, scc)

		if !reflect.DeepEqual(scc, v.scc()) {
			diff := diff.ObjectDiff(scc, v.scc())
			t.Errorf("%s createProvidersFromConstraint mutated constraints. diff:\n%s", k, diff)
		}
		if len(v.expectedErr) > 0 && err == nil {
			t.Errorf("%s expected error %q", k, v.expectedErr)
			continue
		}
		if len(v.expectedErr) == 0 && err != nil {
			t.Errorf("%s did not expect an error but received %v", k, err)
			continue
		}

		// check that we got the error we expected
		if len(v.expectedErr) > 0 {
			if !strings.Contains(err.Error(), v.expectedErr) {
				t.Errorf("%s expected error '%s' but received %v", k, v.expectedErr, err)
			}
		}
	}
}

func TestAssignConstraints(t *testing.T) {
	tests := map[string]struct {
		constraints          []*kapi.SecurityContextConstraints
		namespaceName        string
		client               clientset.Interface
		expectedErrorMessage string
		resolve              func(kscc.SecurityContextConstraintsProvider) (*kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext, error)
		assign               func(kscc.SecurityContextConstraintsProvider, *kapi.SecurityContextConstraints, *kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext) error
	}{
		"no providers": {
			constraints:          []*kapi.SecurityContextConstraints{},
			namespaceName:        "default",
			client:               &fake.Clientset{},
			expectedErrorMessage: "no providers available",
			resolve: func(kscc.SecurityContextConstraintsProvider) (*kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext, error) {
				return nil, nil, nil, nil
			},
			assign: func(kscc.SecurityContextConstraintsProvider, *kapi.SecurityContextConstraints, *kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext) error {
				return nil
			},
		},
		"good assign": {
			constraints: []*kapi.SecurityContextConstraints{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "requiresPreAllocatedUIDRange",
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},

					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},

					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
					Groups: []string{"system:serviceaccounts"},
				},
			},
			namespaceName: "ns",
			client: fake.NewSimpleClientset(&kapi.Namespace{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "ns",
					Annotations: map[string]string{allocator.UIDRangeAnnotation: "1/5"},
				}}),
			resolve: func(kscc.SecurityContextConstraintsProvider) (*kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext, error) {
				return nil, nil, nil, nil
			},
			assign: func(provider kscc.SecurityContextConstraintsProvider, constraint *kapi.SecurityContextConstraints, psc *kapi.PodSecurityContext, a map[string]string, sccs []*kapi.SecurityContext) error {
				if provider != nil && constraint != nil {
					return nil
				}
				return fmt.Errorf("No provider or/and constraint")
			},
		},
		"cannot resolve": {
			constraints: []*kapi.SecurityContextConstraints{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "requiresPreAllocatedUIDRange",
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
					Groups: []string{"system:serviceaccounts"},
				},
			},
			namespaceName: "ns",
			client: fake.NewSimpleClientset(&kapi.Namespace{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "ns",
					Annotations: map[string]string{allocator.UIDRangeAnnotation: "1/5"},
				}}),
			expectedErrorMessage: "unable to resolve security provider: Unable to resolve",
			resolve: func(kscc.SecurityContextConstraintsProvider) (*kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext, error) {
				return nil, nil, nil, fmt.Errorf("Unable to resolve")
			},
			assign: func(kscc.SecurityContextConstraintsProvider, *kapi.SecurityContextConstraints, *kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext) error {
				return nil
			},
		},
		"two sccs": {
			constraints: []*kapi.SecurityContextConstraints{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "badSCCNoFSGroupStrategy",
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
					Groups: []string{"system:serviceaccounts"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "requiresPreAllocatedUIDRange",
					},
					RunAsUser: kapi.RunAsUserStrategyOptions{
						Type: kapi.RunAsUserStrategyMustRunAsRange,
					},
					SELinuxContext: kapi.SELinuxContextStrategyOptions{
						Type: kapi.SELinuxStrategyRunAsAny,
					},
					FSGroup: kapi.FSGroupStrategyOptions{
						Type: kapi.FSGroupStrategyRunAsAny,
					},
					SupplementalGroups: kapi.SupplementalGroupsStrategyOptions{
						Type: kapi.SupplementalGroupsStrategyRunAsAny,
					},
					Groups: []string{"system:serviceaccounts"},
				},
			},
			namespaceName: "ns",
			client: fake.NewSimpleClientset(&kapi.Namespace{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "ns",
					Annotations: map[string]string{allocator.UIDRangeAnnotation: "1/5"},
				}}),
			resolve: func(kscc.SecurityContextConstraintsProvider) (*kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext, error) {
				return nil, nil, nil, nil
			},
			assign: func(kscc.SecurityContextConstraintsProvider, *kapi.SecurityContextConstraints, *kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext) error {
				return nil
			},
		},
	}

	for k, v := range tests {
		err := AssignConstraints(v.constraints, v.namespaceName, v.client, v.resolve, v.assign)
		switch {
		case err == nil && len(v.expectedErrorMessage) == 0:
		case err == nil && len(v.expectedErrorMessage) > 0:
			t.Errorf("%q - Expected error %q, but got no error", k, v.expectedErrorMessage)
		case err != nil && len(v.expectedErrorMessage) == 0:
			t.Errorf("%q - Unexpected error %q", k, err.Error())
		case err.Error() != v.expectedErrorMessage:
			t.Errorf("%q - Unexpected error %q, wanted %q", k, err.Error(), v.expectedErrorMessage)
		}
	}
}
