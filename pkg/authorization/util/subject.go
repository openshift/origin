package util

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/sets"
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

func ExpandSubjects(namespace string, subjects []rbac.Subject) (sets.String, sets.String, []error) {
	var errs []error
	users := sets.String{}
	groups := sets.String{}
	for _, subject := range subjects {
		switch subject.Kind {
		case rbac.UserKind:
			users.Insert(subject.Name)
		case rbac.GroupKind:
			groups.Insert(subject.Name)
		case rbac.ServiceAccountKind:
			// default the namespace to namespace we're working in if
			// it's available. This allows rolebindings that reference
			// SAs in the local namespace to avoid having to qualify
			// them.
			ns := namespace
			if len(subject.Namespace) > 0 {
				ns = subject.Namespace
			}
			if len(ns) >= 0 {
				name := serviceaccount.MakeUsername(ns, subject.Name)
				users.Insert(name)
			}
		default:
			errs = append(errs, fmt.Errorf("Unrecognized kinds for subjects: %s. "+
				"Only UserKind, GroupKind and ServiceAccountKind are supported.", subject.Kind))
		}

	}
	return users, groups, errs
}
