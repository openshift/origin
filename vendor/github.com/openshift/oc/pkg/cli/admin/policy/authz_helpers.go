package policy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

func buildSubjects(users, groups []string) []corev1.ObjectReference {
	subjects := []corev1.ObjectReference{}

	for _, user := range users {
		saNamespace, saName, err := serviceaccount.SplitUsername(user)
		if err == nil {
			subjects = append(subjects, corev1.ObjectReference{Kind: "ServiceAccount", Namespace: saNamespace, Name: saName})
			continue
		}

		subjects = append(subjects, corev1.ObjectReference{Kind: "User", Name: user})
	}

	for _, group := range groups {
		subjects = append(subjects, corev1.ObjectReference{Kind: "Group", Name: group})
	}

	return subjects
}

// stringSubjectsFor returns users and groups for comparison against user.Info.  currentNamespace is used to
// to create usernames for service accounts where namespace=="".
func stringSubjectsFor(currentNamespace string, subjects []corev1.ObjectReference) ([]string, []string) {
	// these MUST be nil to indicate empty
	var users, groups []string

	for _, subject := range subjects {
		switch subject.Kind {
		case "ServiceAccount":
			namespace := currentNamespace
			if len(subject.Namespace) > 0 {
				namespace = subject.Namespace
			}
			if len(namespace) > 0 {
				users = append(users, serviceaccount.MakeUsername(namespace, subject.Name))
			}

		case "User":
			users = append(users, subject.Name)

		case "Group":
			groups = append(groups, subject.Name)
		}
	}

	return users, groups
}
