package certrotation

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

// RotatedSigningCASecret rotates a self-signed signing CA stored in a secret. It creates a new one when
// - refresh duration is over
// - or 80% of validity is over (if RefreshOnlyWhenExpired is false)
// - or the CA is expired.
type RotatedSigningCASecret struct {
	// Namespace is the namespace of the Secret.
	Namespace string
	// Name is the name of the Secret.
	Name string
	// Validity is the duration from time.Now() until the signing CA expires. If RefreshOnlyWhenExpired
	// is false, the signing cert is rotated when 80% of validity is reached.
	Validity time.Duration
	// Refresh is the duration after signing CA creation when it is rotated at the latest. It is ignored
	// if RefreshOnlyWhenExpired is true, or if Refresh > Validity.
	Refresh time.Duration
	// RefreshOnlyWhenExpired set to true means to ignore 80% of validity and the Refresh duration for rotation,
	// but only rotate when the signing CA expires. This is useful for auto-recovery when we want to enforce
	// rotation on expiration only, but not interfere with the ordinary rotation controller.
	RefreshOnlyWhenExpired bool

	// Owner is an optional reference to add to the secret that this rotator creates. Use this when downstream
	// consumers of the signer CA need to be aware of changes to the object.
	// WARNING: be careful when using this option, as deletion of the owning object will cascade into deletion
	// of the signer. If the lifetime of the owning object is not a superset of the lifetime in which the signer
	// is used, early deletion will be catastrophic.
	Owner *metav1.OwnerReference

	// AdditionalAnnotations is a collection of annotations set for the secret
	AdditionalAnnotations AdditionalAnnotations

	// CertificateName is the logical name of this certificate for PKI profile resolution.
	CertificateName string

	// PKIProfileProvider, when non-nil, enables ConfigurablePKI certificate
	// key algorithm resolution. When nil, legacy certificate generation is used.
	PKIProfileProvider pki.PKIProfileProvider

	// Plumbing:
	Informer      corev1informers.SecretInformer
	Lister        corev1listers.SecretLister
	Client        corev1client.SecretsGetter
	EventRecorder events.Recorder
}

// EnsureSigningCertKeyPair manages the entire lifecycle of a signer cert as a secret, from creation to continued rotation.
// It always returns the currently used CA pair, a bool indicating whether it was created/updated within this function call and an error.
func (c RotatedSigningCASecret) EnsureSigningCertKeyPair(ctx context.Context) (*crypto.CA, bool, error) {
	creationRequired := false
	updateRequired := false
	originalSigningCertKeyPairSecret, err := c.Lister.Secrets(c.Namespace).Get(c.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, err
	}
	signingCertKeyPairSecret := originalSigningCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		signingCertKeyPairSecret = &corev1.Secret{
			ObjectMeta: NewTLSArtifactObjectMeta(
				c.Name,
				c.Namespace,
				c.AdditionalAnnotations,
			),
			Type: corev1.SecretTypeTLS,
		}
		creationRequired = true
	}

	// run Update if metadata needs changing unless we're in RefreshOnlyWhenExpired mode
	if !c.RefreshOnlyWhenExpired {
		needsMetadataUpdate := ensureOwnerRefAndTLSAnnotations(signingCertKeyPairSecret, c.Owner, c.AdditionalAnnotations)
		needsTypeChange := ensureSecretTLSTypeSet(signingCertKeyPairSecret)
		updateRequired = needsMetadataUpdate || needsTypeChange
	}

	// run Update if signer content needs changing
	signerUpdated := false
	if needed, reason := c.needNewSigningCertKeyPair(signingCertKeyPairSecret); needed || creationRequired {
		if creationRequired {
			reason = "secret doesn't exist"
		}
		c.EventRecorder.Eventf("SignerUpdateRequired", "%q in %q requires a new signing cert/key pair: %v", c.Name, c.Namespace, reason)
		if err = c.setSigningCertKeyPairSecretAndTLSAnnotations(signingCertKeyPairSecret); err != nil {
			return nil, false, err
		}

		LabelAsManagedSecret(signingCertKeyPairSecret, CertificateTypeSigner)

		updateRequired = true
		signerUpdated = true
	}

	if creationRequired {
		actualSigningCertKeyPairSecret, err := c.Client.Secrets(c.Namespace).Create(ctx, signingCertKeyPairSecret, metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(c.EventRecorder, actualSigningCertKeyPairSecret, err)
		if err != nil {
			return nil, false, err
		}
		klog.V(2).Infof("Created secret %s/%s", actualSigningCertKeyPairSecret.Namespace, actualSigningCertKeyPairSecret.Name)
		signingCertKeyPairSecret = actualSigningCertKeyPairSecret
	} else if updateRequired {
		actualSigningCertKeyPairSecret, err := c.Client.Secrets(c.Namespace).Update(ctx, signingCertKeyPairSecret, metav1.UpdateOptions{})
		if apierrors.IsConflict(err) {
			// ignore error if its attempting to update outdated version of the secret
			return nil, false, nil
		}
		resourcehelper.ReportUpdateEvent(c.EventRecorder, actualSigningCertKeyPairSecret, err)
		if err != nil {
			return nil, false, err
		}
		klog.V(2).Infof("Updated secret %s/%s", actualSigningCertKeyPairSecret.Namespace, actualSigningCertKeyPairSecret.Name)
		signingCertKeyPairSecret = actualSigningCertKeyPairSecret
	}

	// at this point, the secret has the correct signer, so we should read that signer to be able to sign
	signingCertKeyPair, err := crypto.GetCAFromBytes(signingCertKeyPairSecret.Data["tls.crt"], signingCertKeyPairSecret.Data["tls.key"])
	if err != nil {
		return nil, signerUpdated, err
	}

	return signingCertKeyPair, signerUpdated, nil
}

// ensureOwnerReference adds the owner to the list of owner references in meta, if necessary
func ensureOwnerReference(meta *metav1.ObjectMeta, owner *metav1.OwnerReference) bool {
	var found bool
	for _, ref := range meta.OwnerReferences {
		if ref == *owner {
			found = true
			break
		}
	}
	if !found {
		meta.OwnerReferences = append(meta.OwnerReferences, *owner)
		return true
	}
	return false
}

func (c RotatedSigningCASecret) needNewSigningCertKeyPair(secret *corev1.Secret) (bool, string) {
	annotations := secret.Annotations
	notBefore, notAfter, reason := getValidityFromAnnotations(annotations)
	if len(reason) > 0 {
		return true, reason
	}

	if time.Now().After(notAfter) {
		return true, "already expired"
	}

	if c.RefreshOnlyWhenExpired {
		return false, ""
	}

	validity := notAfter.Sub(notBefore)
	at80Percent := notAfter.Add(-validity / 5)
	if time.Now().After(at80Percent) {
		return true, fmt.Sprintf("past refresh time (80%% of validity): %v", at80Percent)
	}

	developerSpecifiedRefresh := notBefore.Add(c.Refresh)
	if time.Now().After(developerSpecifiedRefresh) {
		return true, fmt.Sprintf("past its refresh time %v", developerSpecifiedRefresh)
	}

	return false, ""
}

func getValidityFromAnnotations(annotations map[string]string) (notBefore time.Time, notAfter time.Time, reason string) {
	notAfterString := annotations[CertificateNotAfterAnnotation]
	if len(notAfterString) == 0 {
		return notBefore, notAfter, "missing notAfter"
	}
	notAfter, err := time.Parse(time.RFC3339, notAfterString)
	if err != nil {
		return notBefore, notAfter, fmt.Sprintf("bad expiry: %q", notAfterString)
	}
	notBeforeString := annotations[CertificateNotBeforeAnnotation]
	if len(notBeforeString) == 0 {
		return notBefore, notAfter, "missing notBefore"
	}
	notBefore, err = time.Parse(time.RFC3339, notBeforeString)
	if err != nil {
		return notBefore, notAfter, fmt.Sprintf("bad expiry: %q", notBeforeString)
	}

	return notBefore, notAfter, ""
}

func (c RotatedSigningCASecret) resolveKeyPairGenerator() (crypto.KeyPairGenerator, error) {
	return resolveKeyPairGeneratorWithFallback(c.PKIProfileProvider, pki.CertificateTypeSigner, c.CertificateName)
}

// resolveKeyPairGeneratorWithFallback resolves the key pair generator from the
// PKI profile provider. Returns nil for Unmanaged mode (no key override).
//
// TODO(sanchezl): Remove the fallback to DefaultPKIProfile() once installer
// support for the PKI resource is in place. Until then, the PKI resource may
// not exist in TechPreview clusters. Once removed, callers can use
// pki.ResolveCertificateConfig directly.
func resolveKeyPairGeneratorWithFallback(provider pki.PKIProfileProvider, certType pki.CertificateType, name string) (crypto.KeyPairGenerator, error) {
	cfg, err := pki.ResolveCertificateConfig(provider, certType, name)
	if err != nil {
		klog.Warningf("Failed to resolve PKI config for %s %q, falling back to default profile: %v", certType, name, err)
		defaultProfile := pki.DefaultPKIProfile()
		cfg, err = pki.ResolveCertificateConfig(pki.NewStaticPKIProfileProvider(&defaultProfile), certType, name)
		if err != nil {
			return nil, err
		}
	}
	if cfg == nil {
		return nil, nil
	}
	return cfg.Key, nil
}

// setSigningCertKeyPairSecretAndTLSAnnotations generates a new signing certificate and key pair,
// stores them in the specified secret, and adds predefined TLS annotations to that secret.
func (c RotatedSigningCASecret) setSigningCertKeyPairSecretAndTLSAnnotations(signingCertKeyPairSecret *corev1.Secret) error {
	ca, err := c.setSigningCertKeyPairSecret(signingCertKeyPairSecret)
	if err != nil {
		return err
	}

	c.setTLSAnnotationsOnSigningCertKeyPairSecret(signingCertKeyPairSecret, ca)
	return nil
}

// setSigningCertKeyPairSecret creates a new signing cert/key pair and sets them in the secret.
func (c RotatedSigningCASecret) setSigningCertKeyPairSecret(signingCertKeyPairSecret *corev1.Secret) (*crypto.TLSCertificateConfig, error) {
	signerName := fmt.Sprintf("%s_%s@%d", signingCertKeyPairSecret.Namespace, signingCertKeyPairSecret.Name, time.Now().Unix())

	var ca *crypto.TLSCertificateConfig
	var err error
	if c.PKIProfileProvider != nil {
		keyGen, err := c.resolveKeyPairGenerator()
		if err != nil {
			return nil, err
		}
		if keyGen != nil {
			ca, err = crypto.NewSigningCertificate(signerName, keyGen, crypto.WithLifetime(c.Validity))
			if err != nil {
				return nil, err
			}
		}
		// nil keyGen means Unmanaged: fall through to legacy cert generation
	}
	if ca == nil {
		ca, err = crypto.MakeSelfSignedCAConfigForDuration(signerName, c.Validity)
	}
	if err != nil {
		return nil, err
	}

	certBytes := &bytes.Buffer{}
	keyBytes := &bytes.Buffer{}
	if err = ca.WriteCertConfig(certBytes, keyBytes); err != nil {
		return nil, err
	}

	if signingCertKeyPairSecret.Annotations == nil {
		signingCertKeyPairSecret.Annotations = map[string]string{}
	}
	if signingCertKeyPairSecret.Data == nil {
		signingCertKeyPairSecret.Data = map[string][]byte{}
	}
	signingCertKeyPairSecret.Data["tls.crt"] = certBytes.Bytes()
	signingCertKeyPairSecret.Data["tls.key"] = keyBytes.Bytes()
	return ca, nil
}

// setTLSAnnotationsOnSigningCertKeyPairSecret applies predefined TLS annotations to the given secret.
//
// This function does not perform nil checks on its parameters and assumes that the
// secret's Annotations field has already been initialized.
//
// These assumptions are safe because this function is only called after the secret
// has been initialized in setSigningCertKeyPairSecret.
func (c RotatedSigningCASecret) setTLSAnnotationsOnSigningCertKeyPairSecret(signingCertKeyPairSecret *corev1.Secret, ca *crypto.TLSCertificateConfig) {
	signingCertKeyPairSecret.Annotations[CertificateIssuer] = ca.Certs[0].Issuer.CommonName

	tlsAnnotations := c.AdditionalAnnotations
	tlsAnnotations.NotBefore = ca.Certs[0].NotBefore.Format(time.RFC3339)
	tlsAnnotations.NotAfter = ca.Certs[0].NotAfter.Format(time.RFC3339)
	tlsAnnotations.RefreshPeriod = c.Refresh.String()
	_ = tlsAnnotations.EnsureTLSMetadataUpdate(&signingCertKeyPairSecret.ObjectMeta)
}
