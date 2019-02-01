package certrotation

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/golang/glog"

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

// TargetRotation rotates a key and cert signed by a CA. It creates a new one when <RefreshPercentage>
// of the lifetime of the old cert has passed, or if the common name of the CA changes.
type TargetRotation struct {
	Namespace         string
	Name              string
	Validity          time.Duration
	RefreshPercentage float32

	// Only one of client, serving, or signer rotation may be specified.
	// TODO refactor with an interface for actually signing and move the one-of check higher in the stack.
	ClientRotation  *ClientRotation
	ServingRotation *ServingRotation
	SignerRotation  *SignerRotation

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

type SignerRotation struct {
	SignerName string
}

func (c TargetRotation) ensureTargetCertKeyPair(signingCertKeyPair *crypto.CA, caBundleCerts []*x509.Certificate) error {
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

	if needNewTargetCertKeyPair(targetCertKeyPairSecret.Annotations, signingCertKeyPair, caBundleCerts, c.Validity, c.RefreshPercentage) {
		c.EventRecorder.Eventf("TargetUpdateRequired", "%q in %q requires a new target cert/key pair", c.Name, c.Namespace)
		if err := setTargetCertKeyPairSecret(targetCertKeyPairSecret, c.Validity, signingCertKeyPair, c.ClientRotation, c.ServingRotation, c.SignerRotation); err != nil {
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

func needNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, caBundleCerts []*x509.Certificate, validity time.Duration, renewalPercentage float32) bool {
	if needNewTargetCertKeyPairForTime(annotations, signer, validity, renewalPercentage) {
		return true
	}

	// check the signer common name against all the common names in our ca bundle so we don't refresh early
	signerCommonName := annotations[CertificateSignedBy]
	if len(signerCommonName) == 0 {
		return true
	}
	for _, caCert := range caBundleCerts {
		if signerCommonName == caCert.Subject.CommonName {
			return false
		}
	}

	return true
}

// needNewTargetCertKeyPairForTime returns true when
// 1. there is no targetcrtexpiry indicated in the annotation
// 2. when the targetcertexpiry is malformed
// 3. when now is after the targetcertexpiry
// 4. when now is after timeToRotate(targetcertexpiry - targetValidity*(1-renewalPercentage) AND the signer has been valid
//    for more than 10% of the "extra" time we renew the target
//
//in other words, we rotate if
//
//our old CA is gone from the bundle (then we are pretty late to the renewal party)
//or the cert expired (then we are also pretty late)
//or we are over the renewal percentage of the validity, but only if the new CA at least 10% into its age.
//Maybe worth a go doc.
//
//So in general we need to see a signing CA at least aged 10% within 1-percentage of the cert validity.
//
//Hence, if the CAs are rotated too fast (like CA percentage around 10% or smaller), we will not hit the time to make use of the CA. Or if the cert renewal percentage is at 90%, there is not much time either.
//
//So with a cert percentage of 75% and equally long CA and cert validities at the worst case we start at 85% of the cert to renew, trying again every minute.
func needNewTargetCertKeyPairForTime(annotations map[string]string, signer *crypto.CA, validity time.Duration, renewalPercentage float32) bool {
	targetExpiry := annotations[CertificateExpiryAnnotation]
	if len(targetExpiry) == 0 {
		return true
	}
	certExpiry, err := time.Parse(time.RFC3339, targetExpiry)
	if err != nil {
		glog.Infof("bad expiry: %q", targetExpiry)
		// just create a new one
		return true
	}

	// If Certificate is past its validity, we may must generate new.
	if time.Now().After(certExpiry) {
		return true
	}

	// If Certificate is past its validity*renewpercent, we may have action to take. if the signer is old enough
	renewalDuration := -1 * float32(validity) * (1 - renewalPercentage)
	if time.Now().After(certExpiry.Add(time.Duration(renewalDuration))) {
		// make sure the signer has been valid for more than 10% of the extra renewal time
		timeToWaitForTrustRotation := -1 * renewalDuration / 10
		if time.Now().After(signer.Config.Certs[0].NotBefore.Add(time.Duration(timeToWaitForTrustRotation))) {
			return true
		}
	}

	return false
}

// setTargetCertKeyPairSecret creates a new cert/key pair and sets them in the secret.  Only one of client, serving, or signer rotation may be specified.
// TODO refactor with an interface for actually signing and move the one-of check higher in the stack.
func setTargetCertKeyPairSecret(targetCertKeyPairSecret *corev1.Secret, validity time.Duration, signer *crypto.CA, clientRotation *ClientRotation, servingRotation *ServingRotation, signerRotation *SignerRotation) error {
	numNonNil := 0
	if clientRotation != nil {
		numNonNil++
	}
	if servingRotation != nil {
		numNonNil++
	}
	if signerRotation != nil {
		numNonNil++
	}
	if numNonNil != 1 {
		return fmt.Errorf("exactly one of client, serving, or signing rotation must be specified")
	}

	if targetCertKeyPairSecret.Annotations == nil {
		targetCertKeyPairSecret.Annotations = map[string]string{}
	}
	if targetCertKeyPairSecret.Data == nil {
		targetCertKeyPairSecret.Data = map[string][]byte{}
	}

	// our annotation is based on our cert validity, so we want to make sure that we don't specify something past our signer
	targetValidity := validity
	remainingSignerValidity := signer.Config.Certs[0].NotAfter.Sub(time.Now())
	if remainingSignerValidity < validity {
		targetValidity = remainingSignerValidity
	}

	var certKeyPair *crypto.TLSCertificateConfig
	var err error
	switch {
	case clientRotation != nil:
		certKeyPair, err = signer.MakeClientCertificateForDuration(clientRotation.UserInfo, targetValidity)

	case servingRotation != nil:
		certKeyPair, err = signer.MakeServerCertForDuration(sets.NewString(servingRotation.Hostnames...), targetValidity, servingRotation.CertificateExtensionFn...)

	case signerRotation != nil:
		signerName := fmt.Sprintf("%s_@%d", signerRotation.SignerName, time.Now().Unix())
		certKeyPair, err = crypto.MakeCAConfigForDuration(signerName, validity, signer)
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
