package certrotation

import (
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/operator/events"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/library-go/pkg/crypto"
)

type TargetRotation struct {
	Namespace         string
	Name              string
	Validity          time.Duration
	RefreshPercentage float32

	ClientRotation  *ClientRotation
	ServingRotation *ServingRotation

	Informer      corev1informers.SecretInformer
	Lister        corev1listers.SecretLister
	Client        corev1client.SecretsGetter
	EventRecorder events.Recorder
}

type ClientRotation struct {
	UserInfo user.Info
}

type ServingRotation struct {
	Hostnames              []string
	CertificateExtensionFn []crypto.CertificateExtensionFunc
}

func (c TargetRotation) ensureTargetCertKeyPair(signingCertKeyPair *crypto.CA) error {
	// at this point our trust bundle has been updated.  We don't know for sure that consumers have updated, but that's why we have a second
	// validity percentage.  We always check to see if we need to sign.  Often we are signing with an old key or we have no target
	// and need to mint one
	// TODO do the cross signing thing, but this shows the API consumers want and a very simple impl.
	originalTargetCertKeyPairSecret, err := c.Lister.Secrets(c.Namespace).Get(c.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	targetCertKeyPairSecret := originalTargetCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		targetCertKeyPairSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: c.Namespace, Name: c.Name}}
	}
	targetCertKeyPairSecret.Type = corev1.SecretTypeTLS

	if needNewTargetCertKeyPair(targetCertKeyPairSecret.Annotations, signingCertKeyPair, c.Validity, c.RefreshPercentage) {
		c.EventRecorder.Eventf("TargetUpdateRequired", "%q in %q requires a new target cert/key pair", c.Name, c.Namespace)
		if err := setTargetCertKeyPairSecret(targetCertKeyPairSecret, c.Validity, signingCertKeyPair, c.ClientRotation, c.ServingRotation); err != nil {
			return err
		}

		actualTargetCertKeyPairSecret, err := c.Client.Secrets(c.Namespace).Update(targetCertKeyPairSecret)
		if apierrors.IsNotFound(err) {
			actualTargetCertKeyPairSecret, err = c.Client.Secrets(c.Namespace).Create(targetCertKeyPairSecret)
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
func setTargetCertKeyPairSecret(targetCertKeyPairSecret *corev1.Secret, validity time.Duration, signer *crypto.CA, clientRotation *ClientRotation, servingRotation *ServingRotation) error {
	if (servingRotation != nil) == (clientRotation != nil) {
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
	if servingRotation != nil {
		certKeyPair, err = signer.MakeServerCertForDuration(sets.NewString(servingRotation.Hostnames...), validity, servingRotation.CertificateExtensionFn...)
	} else {
		certKeyPair, err = signer.MakeClientCertificateForDuration(clientRotation.UserInfo, validity)
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
