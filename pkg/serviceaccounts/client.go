package serviceaccounts

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

// IsValidServiceAccountToken returns true if the given secret contains a service account token valid for the given service account
// TODO, these should collapse onto the upstream method
func IsValidServiceAccountToken(serviceAccount *kapi.ServiceAccount, secret *kapi.Secret) bool {
	if secret.Type != kapi.SecretTypeServiceAccountToken {
		return false
	}
	if secret.Namespace != serviceAccount.Namespace {
		return false
	}
	if secret.Annotations[kapi.ServiceAccountNameKey] != serviceAccount.Name {
		return false
	}
	if secret.Annotations[kapi.ServiceAccountUIDKey] != string(serviceAccount.UID) {
		return false
	}
	if len(secret.Data[kapi.ServiceAccountTokenKey]) == 0 {
		return false
	}
	return true
}
