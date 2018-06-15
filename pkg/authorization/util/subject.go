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

func RBACSubjectsToUsersAndGroups(subjects []rbac.Subject, defaultNamespace string) (users []string, groups []string) {
	for _, subject := range subjects {

		switch {
		case subject.APIGroup == rbac.GroupName && subject.Kind == rbac.GroupKind:
			groups = append(groups, subject.Name)
		case subject.APIGroup == rbac.GroupName && subject.Kind == rbac.UserKind:
			users = append(users, subject.Name)
		case subject.APIGroup == "" && subject.Kind == rbac.ServiceAccountKind:
			// default the namespace to namespace we're working in if
			// it's available. This allows rolebindings that reference
			// SAs in the local namespace to avoid having to qualify
			// them.
			ns := defaultNamespace
			if len(subject.Namespace) > 0 {
				ns = subject.Namespace
			}
			if len(ns) > 0 {
				name := serviceaccount.MakeUsername(ns, subject.Name)
				users = append(users, name)
			} else {
				// maybe error?  this fails safe at any rate
			}
		default:
			// maybe error?  This fails safe at any rate
		}
	}

	return users, groups
}
