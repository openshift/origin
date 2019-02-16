package certrotation

import (
	"bytes"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// SigningRotation rotates a self-signed signing CA stored in a secret. It creates a new one when <RefreshPercentage>
// of the lifetime of the old CA has passed.
type SigningRotation struct {
	Namespace         string
	Name              string
	Validity          time.Duration
	RefreshPercentage float32

	Informer      corev1informers.SecretInformer
	Lister        corev1listers.SecretLister
	Client        corev1client.SecretsGetter
	EventRecorder events.Recorder
}

func (c SigningRotation) ensureSigningCertKeyPair() (*crypto.CA, error) {
	originalSigningCertKeyPairSecret, err := c.Lister.Secrets(c.Namespace).Get(c.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	signingCertKeyPairSecret := originalSigningCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		signingCertKeyPairSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: c.Namespace, Name: c.Name}}
	}
	signingCertKeyPairSecret.Type = corev1.SecretTypeTLS

	if reason := needNewSigningCertKeyPair(signingCertKeyPairSecret.Annotations, c.Validity, c.RefreshPercentage); len(reason) > 0 {
		c.EventRecorder.Eventf("SignerUpdateRequired", "%q in %q requires a new signing cert/key pair: %v", c.Name, c.Namespace, reason)
		if err := setSigningCertKeyPairSecret(signingCertKeyPairSecret, c.Validity); err != nil {
			return nil, err
		}

		actualSigningCertKeyPairSecret, _, err := resourceapply.ApplySecret(c.Client, c.EventRecorder, signingCertKeyPairSecret)
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

func needNewSigningCertKeyPair(annotations map[string]string, validity time.Duration, renewalPercentage float32) string {
	signingCertKeyPairExpiry := annotations[CertificateExpiryAnnotation]
	if len(signingCertKeyPairExpiry) == 0 {
		return "missing target expiry"
	}
	certExpiry, err := time.Parse(time.RFC3339, signingCertKeyPairExpiry)
	if err != nil {
		return fmt.Sprintf("bad expiry: %q", signingCertKeyPairExpiry)
	}

	// If Certificate is not-expired, skip this iteration.
	renewalDuration := -1 * float32(validity) * (1 - renewalPercentage)
	if certExpiry.Add(time.Duration(renewalDuration)).After(time.Now()) {
		return ""
	}

	return fmt.Sprintf("past its renewal time %v, versus %v", certExpiry, certExpiry.Add(time.Duration(renewalDuration)))
}

// setSigningCertKeyPairSecret creates a new signing cert/key pair and sets them in the secret
func setSigningCertKeyPairSecret(signingCertKeyPairSecret *corev1.Secret, validity time.Duration) error {
	signerName := fmt.Sprintf("%s_%s@%d", signingCertKeyPairSecret.Namespace, signingCertKeyPairSecret.Name, time.Now().Unix())
	ca, err := crypto.MakeSelfSignedCAConfigForDuration(signerName, validity)
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
