package certgraphanalysis

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
)

type certGenerationOptions interface {
	approved()
}

type certGenerationOptionList []certGenerationOptions

// TODO randomize order of traversal in these functions

func (l certGenerationOptionList) rejectConfigMap(configMap *corev1.ConfigMap) bool {
	for _, curr := range l {
		option, ok := curr.(*resourceFilteringOptions)
		if !ok {
			continue
		}
		if option.rejectConfigMap == nil {
			continue
		}
		if option.rejectConfigMap(configMap) {
			return true
		}
	}
	return false
}

func (l certGenerationOptionList) rejectSecret(secret *corev1.Secret) bool {
	for _, curr := range l {
		option, ok := curr.(*resourceFilteringOptions)
		if !ok {
			continue
		}
		if option.rejectSecret == nil {
			continue
		}
		if option.rejectSecret(secret) {
			return true
		}
	}
	return false
}

func (l certGenerationOptionList) rewriteCABundle(caBundle *certgraphapi.CertificateAuthorityBundle) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteCABundle == nil {
			continue
		}
		option.rewriteCABundle(caBundle)
	}
}

func (l certGenerationOptionList) rewriteCertKeyPair(certKeyPair *certgraphapi.CertKeyPair) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteCertKeyPair == nil {
			continue
		}
		option.rewriteCertKeyPair(certKeyPair)
	}
}
