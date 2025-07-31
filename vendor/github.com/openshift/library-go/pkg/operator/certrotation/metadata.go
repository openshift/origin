package certrotation

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ensureOwnerRefAndTLSAnnotations(secret *corev1.Secret, owner *metav1.OwnerReference, additionalAnnotations AdditionalAnnotations) bool {
	needsMetadataUpdate := false
	// no ownerReference set
	if owner != nil {
		needsMetadataUpdate = ensureOwnerReference(&secret.ObjectMeta, owner)
	}
	// ownership annotations not set
	return additionalAnnotations.EnsureTLSMetadataUpdate(&secret.ObjectMeta) || needsMetadataUpdate
}

func ensureSecretTLSTypeSet(secret *corev1.Secret) bool {
	// Existing secret not found - no need to update metadata (will be done by needNewSigningCertKeyPair / NeedNewTargetCertKeyPair)
	if len(secret.ResourceVersion) == 0 {
		return false
	}

	// convert outdated secret type (created by pre 4.7 installer)
	if secret.Type != corev1.SecretTypeTLS {
		secret.Type = corev1.SecretTypeTLS
		// wipe secret contents if tls.crt and tls.key are missing
		_, certExists := secret.Data[corev1.TLSCertKey]
		_, keyExists := secret.Data[corev1.TLSPrivateKeyKey]
		if !certExists || !keyExists {
			secret.Data = map[string][]byte{}
		}
		return true
	}
	return false
}
