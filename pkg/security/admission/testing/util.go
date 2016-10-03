package testing

import (
	kapi "k8s.io/kubernetes/pkg/api"

	allocator "github.com/openshift/origin/pkg/security"
)

// CreateSAForTest Build and Initializes a ServiceAccount for tests
func CreateSAForTest() *kapi.ServiceAccount {
	return &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
}

// CreateNamespaceForTest builds and initializes a Namespaces for tests
func CreateNamespaceForTest() *kapi.Namespace {
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

// UserScc creates a SCC for a given user name
func UserScc(user string) *kapi.SecurityContextConstraints {
	var uid int64 = 9999
	fsGroup := int64(1)
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/" + user,
			Name:     user,
		},
		Users: []string{user},
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
