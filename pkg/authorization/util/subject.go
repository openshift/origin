package util

import (
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

func BuildRBACSubjects(users, groups []string) []rbac.Subject {
	subjects := []rbac.Subject{}

	for _, user := range users {
		saNamespace, saName, err := serviceaccount.SplitUsername(user)
		if err == nil {
			subjects = append(subjects, rbac.Subject{Kind: rbac.ServiceAccountKind, Namespace: saNamespace, Name: saName})
		} else {
			subjects = append(subjects, rbac.Subject{Kind: rbac.UserKind, APIGroup: rbac.GroupName, Name: user})
		}
	}

	for _, group := range groups {
		subjects = append(subjects, rbac.Subject{Kind: rbac.GroupKind, APIGroup: rbac.GroupName, Name: group})
	}

	return subjects
}
