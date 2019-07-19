package testing

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	securityv1 "github.com/openshift/api/security/v1"
)

// CreateSAForTest Build and Initializes a ServiceAccount for tests
func CreateSAForTest() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
}

// CreateNamespaceForTest builds and initializes a Namespaces for tests
func CreateNamespaceForTest() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				securityv1.UIDRangeAnnotation:           "1/3",
				securityv1.MCSAnnotation:                "s0:c1,c0",
				securityv1.SupplementalGroupsAnnotation: "2/3",
			},
		},
	}
}

// UserScc creates a SCC for a given user name
func UserScc(user string) *securityv1.SecurityContextConstraints {
	var uid int64 = 9999
	fsGroup := int64(1)
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink: "/api/version/securitycontextconstraints/" + user,
			Name:     user,
		},
		Users: []string{user},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAs,
			UID:  &uid,
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{Min: fsGroup, Max: fsGroup},
			},
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
	}
}
