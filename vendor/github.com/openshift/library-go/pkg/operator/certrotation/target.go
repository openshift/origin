package certrotation

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/certs"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// RotatedSelfSignedCertKeySecret rotates a key and cert signed by a signing CA and stores it in a secret.
//
// It creates a new one when
// - refresh duration is over
// - or 80% of validity is over (if RefreshOnlyWhenExpired is false)
// - or the cert is expired.
// - or the signing CA changes.
type RotatedSelfSignedCertKeySecret struct {
	// Namespace is the namespace of the Secret.
	Namespace string
	// Name is the name of the Secret.
	Name string
	// Validity is the duration from time.Now() until the certificate expires. If RefreshOnlyWhenExpired
	// is false, the key and certificate is rotated when 80% of validity is reached.
	Validity time.Duration
	// Refresh is the duration after certificate creation when it is rotated at the latest. It is ignored
	// if RefreshOnlyWhenExpired is true, or if Refresh > Validity.
	// Refresh is ignored until the signing CA at least 10% in its life-time to ensure it is deployed
	// through-out the cluster.
	Refresh time.Duration
	// RefreshOnlyWhenExpired allows rotating only certs that are already expired. (for autorecovery)
	// If false (regular flow) it rotates at the refresh interval but no later then 4/5 of the cert lifetime.

	// RefreshOnlyWhenExpired set to true means to ignore 80% of validity and the Refresh duration for rotation,
	// but only rotate when the certificate expires. This is useful for auto-recovery when we want to enforce
	// rotation on expiration only, but not interfere with the ordinary rotation controller.
	RefreshOnlyWhenExpired bool

	// Owner is an optional reference to add to the secret that this rotator creates. Use this when downstream
	// consumers of the certificate need to be aware of changes to the object.
	// WARNING: be careful when using this option, as deletion of the owning object will cascade into deletion
	// of the certificate. If the lifetime of the owning object is not a superset of the lifetime in which the
	// certificate is used, early deletion will be catastrophic.
	Owner *metav1.OwnerReference

	// AdditionalAnnotations is a collection of annotations set for the secret
	AdditionalAnnotations AdditionalAnnotations

	// CertCreator does the actual cert generation.
	CertCreator TargetCertCreator

	// Plumbing:
	Informer      corev1informers.SecretInformer
	Lister        corev1listers.SecretLister
	Client        corev1client.SecretsGetter
	EventRecorder events.Recorder
}

type TargetCertCreator interface {
	// NewCertificate creates a new key-cert pair with the given signer.
	NewCertificate(signer *crypto.CA, validity time.Duration) (*crypto.TLSCertificateConfig, error)
	// NeedNewTargetCertKeyPair decides whether a new cert-key pair is needed. It returns a non-empty reason if it is the case.
	NeedNewTargetCertKeyPair(currentCertSecret *corev1.Secret, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration, refreshOnlyWhenExpired, creationRequired bool) string
	// SetAnnotations gives an option to override or set additional annotations
	SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string
}

// TargetCertRechecker is an optional interface to be implemented by the TargetCertCreator to enforce
// a controller run.
type TargetCertRechecker interface {
	RecheckChannel() <-chan struct{}
}

func (c RotatedSelfSignedCertKeySecret) EnsureTargetCertKeyPair(ctx context.Context, signingCertKeyPair *crypto.CA, caBundleCerts []*x509.Certificate) (*corev1.Secret, error) {
	// at this point our trust bundle has been updated.  We don't know for sure that consumers have updated, but that's why we have a second
	// validity percentage.  We always check to see if we need to sign.  Often we are signing with an old key or we have no target
	// and need to mint one
	// TODO do the cross signing thing, but this shows the API consumers want and a very simple impl.

	creationRequired := false
	updateRequired := false
	originalTargetCertKeyPairSecret, err := c.Lister.Secrets(c.Namespace).Get(c.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	targetCertKeyPairSecret := originalTargetCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		targetCertKeyPairSecret = &corev1.Secret{
			ObjectMeta: NewTLSArtifactObjectMeta(
				c.Name,
				c.Namespace,
				c.AdditionalAnnotations,
			),
			Type: corev1.SecretTypeTLS,
		}
		creationRequired = true
	}

	needsMetadataUpdate := ensureMetadataUpdate(targetCertKeyPairSecret, c.Owner, c.AdditionalAnnotations)
	needsTypeChange := ensureSecretTLSTypeSet(targetCertKeyPairSecret)
	updateRequired = needsMetadataUpdate || needsTypeChange

	if reason := c.CertCreator.NeedNewTargetCertKeyPair(targetCertKeyPairSecret, signingCertKeyPair, caBundleCerts, c.Refresh, c.RefreshOnlyWhenExpired, creationRequired); len(reason) > 0 {
		c.EventRecorder.Eventf("TargetUpdateRequired", "%q in %q requires a new target cert/key pair: %v", c.Name, c.Namespace, reason)
		if err := setTargetCertKeyPairSecret(targetCertKeyPairSecret, c.Validity, signingCertKeyPair, c.CertCreator, c.AdditionalAnnotations); err != nil {
			return nil, err
		}

		LabelAsManagedSecret(targetCertKeyPairSecret, CertificateTypeTarget)

		updateRequired = true
	}
	if creationRequired {
		actualTargetCertKeyPairSecret, err := c.Client.Secrets(c.Namespace).Create(ctx, targetCertKeyPairSecret, metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(c.EventRecorder, actualTargetCertKeyPairSecret, err)
		if err != nil {
			return nil, err
		}
		klog.V(2).Infof("Created secret %s/%s", actualTargetCertKeyPairSecret.Namespace, actualTargetCertKeyPairSecret.Name)
		targetCertKeyPairSecret = actualTargetCertKeyPairSecret
	} else if updateRequired {
		actualTargetCertKeyPairSecret, err := c.Client.Secrets(c.Namespace).Update(ctx, targetCertKeyPairSecret, metav1.UpdateOptions{})
		resourcehelper.ReportUpdateEvent(c.EventRecorder, actualTargetCertKeyPairSecret, err)
		if err != nil {
			return nil, err
		}
		klog.V(2).Infof("Updated secret %s/%s", actualTargetCertKeyPairSecret.Namespace, actualTargetCertKeyPairSecret.Name)
		targetCertKeyPairSecret = actualTargetCertKeyPairSecret
	}

	return targetCertKeyPairSecret, nil
}

func needNewTargetCertKeyPair(secret *corev1.Secret, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration, refreshOnlyWhenExpired, creationRequired bool) string {
	if creationRequired {
		return "secret doesn't exist"
	}

	annotations := secret.Annotations
	if reason := needNewTargetCertKeyPairForTime(annotations, signer, refresh, refreshOnlyWhenExpired); len(reason) > 0 {
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
//  1. when notAfter or notBefore is missing in the annotation
//  2. when notAfter or notBefore is malformed
//  3. when now is after the notAfter
//  4. when now is after notAfter+refresh AND the signer has been valid
//     for more than 5% of the "extra" time we renew the target
//
// in other words, we rotate if
//
// our old CA is gone from the bundle (then we are pretty late to the renewal party)
// or the cert expired (then we are also pretty late)
// or we are over the renewal percentage of the validity, but only if the new CA at least 10% into its age.
// Maybe worth a go doc.
//
// So in general we need to see a signing CA at least aged 10% within 1-percentage of the cert validity.
//
// Hence, if the CAs are rotated too fast (like CA percentage around 10% or smaller), we will not hit the time to make use of the CA. Or if the cert renewal percentage is at 90%, there is not much time either.
//
// So with a cert percentage of 75% and equally long CA and cert validities at the worst case we start at 85% of the cert to renew, trying again every minute.
func needNewTargetCertKeyPairForTime(annotations map[string]string, signer *crypto.CA, refresh time.Duration, refreshOnlyWhenExpired bool) string {
	notBefore, notAfter, reason := getValidityFromAnnotations(annotations)
	if len(reason) > 0 {
		return reason
	}

	// Is cert expired?
	if time.Now().After(notAfter) {
		return "already expired"
	}

	if refreshOnlyWhenExpired {
		return ""
	}

	// Are we at 80% of validity?
	validity := notAfter.Sub(notBefore)
	at80Percent := notAfter.Add(-validity / 5)
	if time.Now().After(at80Percent) {
		return fmt.Sprintf("past refresh time (80%% of validity): %v", at80Percent)
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
func setTargetCertKeyPairSecret(targetCertKeyPairSecret *corev1.Secret, validity time.Duration, signer *crypto.CA, certCreator TargetCertCreator, annotations AdditionalAnnotations) error {
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

	_ = annotations.EnsureTLSMetadataUpdate(&targetCertKeyPairSecret.ObjectMeta)
	certCreator.SetAnnotations(certKeyPair, targetCertKeyPairSecret.Annotations)

	return nil
}

type ClientRotation struct {
	UserInfo user.Info
}

func (r *ClientRotation) NewCertificate(signer *crypto.CA, validity time.Duration) (*crypto.TLSCertificateConfig, error) {
	return signer.MakeClientCertificateForDuration(r.UserInfo, validity)
}

func (r *ClientRotation) NeedNewTargetCertKeyPair(currentCertSecret *corev1.Secret, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration, refreshOnlyWhenExpired, exists bool) string {
	return needNewTargetCertKeyPair(currentCertSecret, signer, caBundleCerts, refresh, refreshOnlyWhenExpired, exists)
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
	return signer.MakeServerCertForDuration(sets.New(r.Hostnames()...), validity, r.CertificateExtensionFn...)
}

func (r *ServingRotation) RecheckChannel() <-chan struct{} {
	return r.HostnamesChanged
}

func (r *ServingRotation) NeedNewTargetCertKeyPair(currentCertSecret *corev1.Secret, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration, refreshOnlyWhenExpired, creationRequired bool) string {
	reason := needNewTargetCertKeyPair(currentCertSecret, signer, caBundleCerts, refresh, refreshOnlyWhenExpired, creationRequired)
	if len(reason) > 0 {
		return reason
	}

	return r.missingHostnames(currentCertSecret.Annotations)
}

func (r *ServingRotation) missingHostnames(annotations map[string]string) string {
	existingHostnames := sets.New(strings.Split(annotations[CertificateHostnames], ",")...)
	requiredHostnames := sets.New(r.Hostnames()...)
	if !existingHostnames.Equal(requiredHostnames) {
		existingNotRequired := existingHostnames.Difference(requiredHostnames)
		requiredNotExisting := requiredHostnames.Difference(existingHostnames)
		return fmt.Sprintf("%q are existing and not required, %q are required and not existing", strings.Join(sets.List(existingNotRequired), ","), strings.Join(sets.List(requiredNotExisting), ","))
	}

	return ""
}

func (r *ServingRotation) SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string {
	hostnames := sets.Set[string]{}
	for _, ip := range cert.Certs[0].IPAddresses {
		hostnames.Insert(ip.String())
	}
	for _, dnsName := range cert.Certs[0].DNSNames {
		hostnames.Insert(dnsName)
	}

	// List does a sort so that we have a consistent representation
	annotations[CertificateHostnames] = strings.Join(sets.List(hostnames), ",")
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

func (r *SignerRotation) NeedNewTargetCertKeyPair(currentCertSecret *corev1.Secret, signer *crypto.CA, caBundleCerts []*x509.Certificate, refresh time.Duration, refreshOnlyWhenExpired, exists bool) string {
	return needNewTargetCertKeyPair(currentCertSecret, signer, caBundleCerts, refresh, refreshOnlyWhenExpired, exists)
}

func (r *SignerRotation) SetAnnotations(cert *crypto.TLSCertificateConfig, annotations map[string]string) map[string]string {
	return annotations
}
