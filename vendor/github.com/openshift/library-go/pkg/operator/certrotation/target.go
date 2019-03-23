package certrotation

import (
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/library-go/pkg/certs"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// TargetRotation rotates a key and cert signed by a CA. It creates a new one when <RefreshPercentage>
// of the lifetime of the old cert has passed, or if the common name of the CA changes.
type TargetRotation struct {
	Namespace string
	Name      string
	Validity  time.Duration
	Refresh   time.Duration

	CertCreator TargetCertCreator

	Informer      corev1informers.SecretInformer
	Lister        corev1listers.SecretLister
	Client        corev1client.SecretsGetter
	EventRecorder events.Recorder
}

type TargetCertCreator interface {
	NewCertificate(signer *crypto.CA, validity time.Duration) (*crypto.TLSCertificateConfig, error)
	NeedNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration) string
	// SetAnnotations gives an option to override or set additional annotations
	SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string
}

type TargetCertRechecker interface {
	RecheckChannel() <-chan struct{}
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

	if reason := needNewTargetCertKeyPair(targetCertKeyPairSecret.Annotations, signingCertKeyPair, caBundleCerts, c.Refresh); len(reason) > 0 {
		c.EventRecorder.Eventf("TargetUpdateRequired", "%q in %q requires a new target cert/key pair: %v", c.Name, c.Namespace, reason)
		if err := setTargetCertKeyPairSecret(targetCertKeyPairSecret, c.Validity, signingCertKeyPair, c.CertCreator); err != nil {
			return err
		}

		actualTargetCertKeyPairSecret, _, err := resourceapply.ApplySecret(c.Client, c.EventRecorder, targetCertKeyPairSecret)
		if err != nil {
			return err
		}
		targetCertKeyPairSecret = actualTargetCertKeyPairSecret
	}

	return nil
}

func needNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration) string {
	if reason := needNewTargetCertKeyPairForTime(annotations, signer, refresh); len(reason) > 0 {
		return reason
	}

	// check the signer common name against all the common names in our ca bundle so we don't refresh early
	signerCommonName := annotations[CertificateIssuer]
	if len(signerCommonName) == 0 {
		return "missing issuer name"
	}
	for _, caCert := range caBundleCerts {
		if signerCommonName == caCert.Subject.CommonName {
			return ""
		}
	}

	return fmt.Sprintf("issuer %q, not in ca bundle:\n%s", signerCommonName, certs.CertificateBundleToString(caBundleCerts))
}

// needNewTargetCertKeyPairForTime returns true when
// 1. when notAfter or notBefore is missing in the annotation
// 2. when notAfter or notBefore is malformed
// 3. when now is after the notAfter
// 4. when now is after notAfter+refresh AND the signer has been valid
//    for more than 5% of the "extra" time we renew the target
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
func needNewTargetCertKeyPairForTime(annotations map[string]string, signer *crypto.CA, refresh time.Duration) string {
	notBefore, notAfter, reason := getValidityFromAnnotations(annotations)
	if len(reason) > 0 {
		return reason
	}

	maxWait := notAfter.Sub(notBefore) / 5
	latestTime := notAfter.Add(-maxWait)
	if time.Now().After(latestTime) {
		return fmt.Sprintf("past its latest possible time %v", latestTime)
	}

	// If Certificate is past its refresh time, we may have action to take. We only do this if the signer is old enough.
	refreshTime := notBefore.Add(refresh)
	if time.Now().After(refreshTime) {
		// make sure the signer has been valid for more than 10% of the target's refresh time.
		timeToWaitForTrustRotation := refresh / 10
		if time.Now().After(signer.Config.Certs[0].NotBefore.Add(time.Duration(timeToWaitForTrustRotation))) {
			return fmt.Sprintf("past its refresh time %v", refreshTime)
		}
	}

	return ""
}

// setTargetCertKeyPairSecret creates a new cert/key pair and sets them in the secret.  Only one of client, serving, or signer rotation may be specified.
// TODO refactor with an interface for actually signing and move the one-of check higher in the stack.
func setTargetCertKeyPairSecret(targetCertKeyPairSecret *corev1.Secret, validity time.Duration, signer *crypto.CA, certCreator TargetCertCreator) error {
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

	certKeyPair, err := certCreator.NewCertificate(signer, targetValidity)
	if err != nil {
		return err
	}

	targetCertKeyPairSecret.Data["tls.crt"], targetCertKeyPairSecret.Data["tls.key"], err = certKeyPair.GetPEMBytes()
	if err != nil {
		return err
	}
	targetCertKeyPairSecret.Annotations[CertificateNotAfterAnnotation] = certKeyPair.Certs[0].NotAfter.Format(time.RFC3339)
	targetCertKeyPairSecret.Annotations[CertificateNotBeforeAnnotation] = certKeyPair.Certs[0].NotBefore.Format(time.RFC3339)
	targetCertKeyPairSecret.Annotations[CertificateIssuer] = certKeyPair.Certs[0].Issuer.CommonName
	certCreator.SetAnnotations(certKeyPair, targetCertKeyPairSecret.Annotations)

	return nil
}

type ClientRotation struct {
	UserInfo user.Info
}

func (r *ClientRotation) NewCertificate(signer *crypto.CA, validity time.Duration) (*crypto.TLSCertificateConfig, error) {
	return signer.MakeClientCertificateForDuration(r.UserInfo, validity)
}

func (r *ClientRotation) NeedNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration) string {
	return needNewTargetCertKeyPair(annotations, signer, caBundleCerts, refresh)
}

func (r *ClientRotation) SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string {
	return annotations
}

type ServingRotation struct {
	Hostnames              ServingHostnameFunc
	CertificateExtensionFn []crypto.CertificateExtensionFunc
	HostnamesChanged       <-chan struct{}
}

func (r *ServingRotation) NewCertificate(signer *crypto.CA, validity time.Duration) (*crypto.TLSCertificateConfig, error) {
	if len(r.Hostnames()) == 0 {
		return nil, fmt.Errorf("no hostnames set")
	}
	return signer.MakeServerCertForDuration(sets.NewString(r.Hostnames()...), validity, r.CertificateExtensionFn...)
}

func (r *ServingRotation) RecheckChannel() <-chan struct{} {
	return r.HostnamesChanged
}

func (r *ServingRotation) NeedNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration) string {
	reason := needNewTargetCertKeyPair(annotations, signer, caBundleCerts, refresh)
	if len(reason) > 0 {
		return reason
	}

	return r.missingHostnames(annotations)
}

func (r *ServingRotation) missingHostnames(annotations map[string]string) string {
	existingHostnames := sets.NewString(strings.Split(annotations[CertificateHostnames], ",")...)
	requiredHostnames := sets.NewString(r.Hostnames()...)
	if !existingHostnames.Equal(requiredHostnames) {
		existingNotRequired := existingHostnames.Difference(requiredHostnames)
		requiredNotExisting := requiredHostnames.Difference(existingHostnames)
		return fmt.Sprintf("%q are existing and not required, %q are required and not existing", strings.Join(existingNotRequired.List(), ","), strings.Join(requiredNotExisting.List(), ","))
	}

	return ""
}

func (r *ServingRotation) SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string {
	hostnames := sets.String{}
	for _, ip := range cert.Certs[0].IPAddresses {
		hostnames.Insert(ip.String())
	}
	for _, dnsName := range cert.Certs[0].DNSNames {
		hostnames.Insert(dnsName)
	}

	// List does a sort so that we have a consistent representation
	annotations[CertificateHostnames] = strings.Join(hostnames.List(), ",")
	return annotations
}

type ServingHostnameFunc func() []string

type SignerRotation struct {
	SignerName string
}

func (r *SignerRotation) NewCertificate(signer *crypto.CA, validity time.Duration) (*crypto.TLSCertificateConfig, error) {
	signerName := fmt.Sprintf("%s_@%d", r.SignerName, time.Now().Unix())
	return crypto.MakeCAConfigForDuration(signerName, validity, signer)
}

func (r *SignerRotation) NeedNewTargetCertKeyPair(annotations map[string]string, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration) string {
	return needNewTargetCertKeyPair(annotations, signer, caBundleCerts, refresh)
}

func (r *SignerRotation) SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string {
	return annotations
}
