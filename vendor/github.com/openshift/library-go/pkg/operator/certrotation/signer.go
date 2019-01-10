package certrotation

import (
	"bytes"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/library-go/pkg/crypto"
)

func (c CertRotationController) ensureSigningCertKeyPair() (*crypto.CA, error) {
	originalSigningCertKeyPairSecret, err := c.signingLister.Secrets(c.signingNamespace).Get(c.signingCertKeyPairSecretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	signingCertKeyPairSecret := originalSigningCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		signingCertKeyPairSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: c.signingNamespace, Name: c.signingCertKeyPairSecretName}}
	}
	if needNewSigningCertKeyPair(signingCertKeyPairSecret.Annotations, c.signingCertKeyPairValidity, c.newSigningPercentage) {
		c.eventRecorder.Eventf("SignerUpdateRequired", "%q in %q requires a new signing cert/key pair", c.signingCertKeyPairSecretName, c.signingNamespace)
		if err := setSigningCertKeyPairSecret(signingCertKeyPairSecret, c.signingCertKeyPairValidity); err != nil {
			return nil, err
		}

		actualSigningCertKeyPairSecret, err := c.secretsClient.Secrets(c.signingNamespace).Update(signingCertKeyPairSecret)
		if apierrors.IsNotFound(err) {
			actualSigningCertKeyPairSecret, err = c.secretsClient.Secrets(c.signingNamespace).Create(signingCertKeyPairSecret)
			if err != nil {
				return nil, err
			}
		}
		if err != nil {
			return nil, err
		}
		signingCertKeyPairSecret = actualSigningCertKeyPairSecret
	}
	// at this point, the secret has the correct signer, so we should read that signer to be able to sign
	signingCertKeyPair, err := crypto.GetCAFromBytes(signingCertKeyPairSecret.Data["tls.crt"], signingCertKeyPairSecret.Data["tls.key"])
	if err != nil {
		return nil, err
	}

	return signingCertKeyPair, nil
}

func needNewSigningCertKeyPair(annotations map[string]string, validity time.Duration, renewalPercentage float32) bool {
	return needNewCertKeyPairForTime(annotations, validity, renewalPercentage)
}

// setSigningCertKeyPairSecret creates a new signing cert/key pair and sets them in the secret
func setSigningCertKeyPairSecret(signingCertKeyPairSecret *corev1.Secret, validity time.Duration) error {
	signerName := fmt.Sprintf("%s_%s@%d", signingCertKeyPairSecret.Namespace, signingCertKeyPairSecret.Name, time.Now().Unix())
	ca, err := crypto.MakeCAConfigForDuration(signerName, validity)
	if err != nil {
		return err
	}

	certBytes := &bytes.Buffer{}
	keyBytes := &bytes.Buffer{}
	if err := ca.WriteCertConfig(certBytes, keyBytes); err != nil {
		return err
	}

	if signingCertKeyPairSecret.Annotations == nil {
		signingCertKeyPairSecret.Annotations = map[string]string{}
	}
	if signingCertKeyPairSecret.Data == nil {
		signingCertKeyPairSecret.Data = map[string][]byte{}
	}
	signingCertKeyPairSecret.Data["tls.crt"] = certBytes.Bytes()
	signingCertKeyPairSecret.Data["tls.key"] = keyBytes.Bytes()
	signingCertKeyPairSecret.Annotations[CertificateExpiryAnnotation] = ca.Certs[0].NotAfter.Format(time.RFC3339)

	return nil
}
