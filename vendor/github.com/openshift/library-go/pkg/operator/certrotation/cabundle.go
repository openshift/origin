package certrotation

import (
	"crypto/x509"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"

	"github.com/openshift/library-go/pkg/crypto"
)

func (c CertRotationController) ensureConfigMapCABundle(signingCertKeyPair *crypto.CA) error {
	// by this point we have current signing cert/key pair.  We now need to make sure that the ca-bundle configmap has this cert and
	// doesn't have any expired certs
	originalCABundleConfigMap, err := c.caBundleLister.ConfigMaps(c.caBundleNamespace).Get(c.caBundleConfigMapName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	caBundleConfigMap := originalCABundleConfigMap.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		caBundleConfigMap = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: c.caBundleNamespace, Name: c.caBundleConfigMapName}}
	}
	if err := manageCABundleConfigMap(caBundleConfigMap, signingCertKeyPair.Config.Certs[0]); err != nil {
		return err
	}
	if originalCABundleConfigMap == nil || originalCABundleConfigMap.Data == nil || !equality.Semantic.DeepEqual(originalCABundleConfigMap.Data, caBundleConfigMap.Data) {
		c.eventRecorder.Eventf("CABundleUpdateRequired", "%q in %q requires a new cert", c.caBundleNamespace, c.caBundleConfigMapName)
		actualCABundleConfigMap, err := c.configmapsClient.ConfigMaps(c.caBundleNamespace).Update(caBundleConfigMap)
		if apierrors.IsNotFound(err) {
			actualCABundleConfigMap, err = c.configmapsClient.ConfigMaps(c.caBundleNamespace).Create(caBundleConfigMap)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		caBundleConfigMap = actualCABundleConfigMap
	}

	return nil
}

// manageCABundleConfigMap adds the new certificate to the list of cabundles, eliminates duplicates, and prunes the list of expired
// certs to trust as signers
func manageCABundleConfigMap(caBundleConfigMap *corev1.ConfigMap, currentSigner *x509.Certificate) error {
	if caBundleConfigMap.Data == nil {
		caBundleConfigMap.Data = map[string]string{}
	}

	certificates := []*x509.Certificate{}
	caBundle := caBundleConfigMap.Data["ca-bundle.crt"]
	if len(caBundle) > 0 {
		var err error
		certificates, err = cert.ParseCertsPEM([]byte(caBundle))
		if err != nil {
			return err
		}
	}
	certificates = append([]*x509.Certificate{currentSigner}, certificates...)
	certificates = crypto.FilterExpiredCerts(certificates...)

	finalCertificates := []*x509.Certificate{}
	// now check for duplicates. n^2, but super simple
	for i := range certificates {
		found := false
		for j := range finalCertificates {
			if reflect.DeepEqual(certificates[i].Raw, finalCertificates[j].Raw) {
				found = true
				break
			}
		}
		if !found {
			finalCertificates = append(finalCertificates, certificates[i])
		}
	}

	caBytes, err := crypto.EncodeCertificates(finalCertificates...)
	if err != nil {
		return err
	}
	caBundleConfigMap.Data["ca-bundle.crt"] = string(caBytes)

	return nil
}
