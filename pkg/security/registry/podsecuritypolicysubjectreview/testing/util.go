package testing

import (
	kapi "k8s.io/kubernetes/pkg/api"

	allocator "github.com/openshift/origin/pkg/security"
)

// NewSA Build and Initializes a ServiceAccount for tests
func NewSA() *kapi.ServiceAccount {
	return &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
		},
	}
}

// NewNamespace builds and initializes a Namespaces for tests
func NewNamespace() *kapi.Namespace {
	return &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation:           "1/3",
				allocator.MCSAnnotation:                "s0:c1,c0",
				allocator.SupplementalGroupsAnnotation: "2/3",
			},
		},
	}
}

// UserFooScc creates a SCC for user foo
func UserFooScc() *kapi.SecurityContextConstraints {
	var uid int64 = 9999
	fsGroup := int64(1)
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/foo",
			Name:     "foo",
		},
		Users: []string{"foo"},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &uid,
		},
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
}

// UserBarScc creates a SCC for user bar
func UserBarScc() *kapi.SecurityContextConstraints {
	var uid int64 = 9998
	fsGroup := int64(1)
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/bar",
			Name:     "bar",
		},
		Users: []string{"bar"},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &uid,
		},
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
}
