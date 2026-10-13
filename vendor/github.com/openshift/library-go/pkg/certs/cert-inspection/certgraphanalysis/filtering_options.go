package certgraphanalysis

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type certGenerationOptions interface {
	approved()
}

type annotationSpecifier interface {
	annotationsToCollect() []string
}

type configMapFilter interface {
	rejectConfigMap(configMap *corev1.ConfigMap) bool
}

type secretFilter interface {
	rejectSecret(secret *corev1.Secret) bool
}

type caBundleMetadataRewriter interface {
	rewriteCABundle(metadata metav1.ObjectMeta, caBundle *certgraphapi.CertificateAuthorityBundle)
}

type certKeypairMetadataRewriter interface {
	rewriteCertKeyPair(metadata metav1.ObjectMeta, certKeyPair *certgraphapi.CertKeyPair)
}

type configMapRewriter interface {
	rewriteConfigMap(configMap *corev1.ConfigMap)
}

type secretRewriter interface {
	rewriteSecret(secret *corev1.Secret)
}

type pathRewriter interface {
	rewritePath(path string) string
}

type certGenerationOptionList []certGenerationOptions

// TODO randomize order of traversal in these functions

func (l certGenerationOptionList) annotationsToCollect() []string {
	ret := []string{}
	for _, curr := range l {
		option, ok := curr.(annotationSpecifier)
		if !ok {
			continue
		}
		ret = append(ret, option.annotationsToCollect()...)
	}
	return ret
}

func (l certGenerationOptionList) rejectConfigMap(configMap *corev1.ConfigMap) bool {
	for _, curr := range l {
		option, ok := curr.(configMapFilter)
		if !ok {
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
		option, ok := curr.(secretFilter)
		if !ok {
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
		option, ok := curr.(caBundleMetadataRewriter)
		if !ok {
			continue
		}
		option.rewriteCABundle(metadata, caBundle)
	}
}

func (l certGenerationOptionList) rewriteCertKeyPair(metadata metav1.ObjectMeta, certKeyPair *certgraphapi.CertKeyPair) {
	for _, curr := range l {
		option, ok := curr.(certKeypairMetadataRewriter)
		if !ok {
			continue
		}
		option.rewriteCertKeyPair(metadata, certKeyPair)
	}
}

func (l certGenerationOptionList) rewriteConfigMap(configMap *corev1.ConfigMap) {
	for _, curr := range l {
		option, ok := curr.(configMapRewriter)
		if !ok {
			continue
		}
		option.rewriteConfigMap(configMap)
	}
}

func (l certGenerationOptionList) rewriteSecret(secret *corev1.Secret) {
	for _, curr := range l {
		option, ok := curr.(secretRewriter)
		if !ok {
			continue
		}
		option.rewriteSecret(secret)
	}
}

func (l certGenerationOptionList) rewritePath(path string) string {
	res := path
	for _, curr := range l {
		option, ok := curr.(pathRewriter)
		if !ok {
			continue
		}
		res = option.rewritePath(res)
	}
	return res
}
