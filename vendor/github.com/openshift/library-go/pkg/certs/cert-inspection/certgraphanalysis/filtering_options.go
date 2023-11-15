package certgraphanalysis

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (l certGenerationOptionList) rewriteCABundle(metadata metav1.ObjectMeta, caBundle *certgraphapi.CertificateAuthorityBundle) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteCABundle == nil {
			continue
		}
		option.rewriteCABundle(metadata, caBundle)
	}
}

func (l certGenerationOptionList) rewriteCertKeyPair(metadata metav1.ObjectMeta, certKeyPair *certgraphapi.CertKeyPair) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteCertKeyPair == nil {
			continue
		}
		option.rewriteCertKeyPair(metadata, certKeyPair)
	}
}

func (l certGenerationOptionList) rewriteConfigMap(configMap *corev1.ConfigMap) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteConfigMap == nil {
			continue
		}
		option.rewriteConfigMap(configMap)
	}
}

func (l certGenerationOptionList) rewriteSecret(secret *corev1.Secret) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteSecret == nil {
			continue
		}
		option.rewriteSecret(secret)
	}
}
