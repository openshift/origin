package certrotation

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/library-go/pkg/crypto"
)

func (c CertRotationController) ensureTargetCertKeyPair(signingCertKeyPair *crypto.CA) error {
	// at this point our trust bundle has been updated.  We don't know for sure that consumers have updated, but that's why we have a second
	// validity percentage.  We always check to see if we need to sign.  Often we are signing with an old key or we have no target
	// and need to mint one
	// TODO do the cross signing thing, but this shows the API consumers want and a very simple impl.
	originalTargetCertKeyPairSecret, err := c.targetLister.Secrets(c.targetNamespace).Get(c.targetCertKeyPairSecretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	targetCertKeyPairSecret := originalTargetCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		targetCertKeyPairSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: c.targetNamespace, Name: c.targetCertKeyPairSecretName}}
	}
	if needNewTargetCertKeyPair(targetCertKeyPairSecret.Annotations, signingCertKeyPair, c.targetCertKeyPairValidity, c.newTargetPercentage) {
		c.eventRecorder.Eventf("TargetUpdateRequired", "%q in %q requires a new target cert/key pair", c.targetCertKeyPairSecretName, c.targetNamespace)
		if err := setTargetCertKeyPairSecret(targetCertKeyPairSecret, c.targetCertKeyPairValidity, signingCertKeyPair, c.targetUserInfo, c.targetServingHostnames, c.targetServingCertificateExtensionFn...); err != nil {
			return err
		}

		actualTargetCertKeyPairSecret, err := c.secretsClient.Secrets(c.targetNamespace).Update(targetCertKeyPairSecret)
		if apierrors.IsNotFound(err) {
			actualTargetCertKeyPairSecret, err = c.secretsClient.Secrets(c.targetNamespace).Create(targetCertKeyPairSecret)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		targetCertKeyPairSecret = actualTargetCertKeyPairSecret
	}

	return nil
}

func needNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, validity time.Duration, renewalPercentage float32) bool {
	if needNewCertKeyPairForTime(annotations, validity, renewalPercentage) {
		return true
	}
	signerCommonName := annotations[CertificateSignedBy]
	if len(signerCommonName) == 0 {
		return true
	}
	if signerCommonName != signer.Config.Certs[0].Subject.CommonName {
		return true
	}

	return false
}

// setTargetCertKeyPairSecret creates a new cert/key pair and sets them in the secret
func setTargetCertKeyPairSecret(targetCertKeyPairSecret *corev1.Secret, validity time.Duration, signer *crypto.CA, user user.Info, servingHostnames []string, servingCertificateExtensionFn ...crypto.CertificateExtensionFunc) error {
	if len(servingHostnames) > 0 && user != nil {
		return fmt.Errorf("must be one of server or client cert")
	}
	if targetCertKeyPairSecret.Annotations == nil {
		targetCertKeyPairSecret.Annotations = map[string]string{}
	}
	if targetCertKeyPairSecret.Data == nil {
		targetCertKeyPairSecret.Data = map[string][]byte{}
	}

	var certKeyPair *crypto.TLSCertificateConfig
	var err error
	if len(servingHostnames) > 0 {
		certKeyPair, err = signer.MakeServerCertForDuration(sets.NewString(servingHostnames...), validity, servingCertificateExtensionFn...)
	} else {
		certKeyPair, err = signer.MakeClientCertificateForDuration(user, validity)
	}
	if err != nil {
		return err
	}

	targetCertKeyPairSecret.Data["tls.crt"], targetCertKeyPairSecret.Data["tls.key"], err = certKeyPair.GetPEMBytes()
	if err != nil {
		return err
	}
	targetCertKeyPairSecret.Annotations[CertificateExpiryAnnotation] = certKeyPair.Certs[0].NotAfter.Format(time.RFC3339)
	targetCertKeyPairSecret.Annotations[CertificateSignedBy] = signer.Config.Certs[0].Subject.CommonName

	return nil
}
