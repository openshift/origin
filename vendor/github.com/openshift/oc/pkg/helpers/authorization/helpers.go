package authorization

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	userKind           = "User"
	groupKind          = "Group"
	serviceAccountKind = "ServiceAccount"
	systemUserKind     = "SystemUser"
	systemGroupKind    = "SystemGroup"
)

// SubjectsStrings returns users, groups, serviceaccounts, unknown for display purposes.  currentNamespace is used to
// hide the subject.Namespace for ServiceAccounts in the currentNamespace
func SubjectsStrings(currentNamespace string, subjects []corev1.ObjectReference) ([]string, []string, []string, []string) {
	users := []string{}
	groups := []string{}
	sas := []string{}
	others := []string{}

	for _, subject := range subjects {
		switch subject.Kind {
		case serviceAccountKind:
			if len(subject.Namespace) > 0 && currentNamespace != subject.Namespace {
				sas = append(sas, subject.Namespace+"/"+subject.Name)
			} else {
				sas = append(sas, subject.Name)
			}

		case userKind, systemUserKind:
			users = append(users, subject.Name)

		case groupKind, systemGroupKind:
			groups = append(groups, subject.Name)

		default:
			others = append(others, fmt.Sprintf("%s/%s/%s", subject.Kind, subject.Namespace, subject.Name))

		}
	}

	return users, groups, sas, others
}
