package certrotation

import (
	"github.com/google/go-cmp/cmp"
	"github.com/openshift/api/annotations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	// CertificateNotBeforeAnnotation contains the certificate expiration date in RFC3339 format.
	CertificateNotBeforeAnnotation = "auth.openshift.io/certificate-not-before"
	// CertificateNotAfterAnnotation contains the certificate expiration date in RFC3339 format.
	CertificateNotAfterAnnotation = "auth.openshift.io/certificate-not-after"
	// CertificateIssuer contains the common name of the certificate that signed another certificate.
	CertificateIssuer = "auth.openshift.io/certificate-issuer"
	// CertificateHostnames contains the hostnames used by a signer.
	CertificateHostnames = "auth.openshift.io/certificate-hostnames"
	// CertificateTestNameAnnotation is an e2e test name which verifies that TLS artifact is created and used correctly
	CertificateTestNameAnnotation string = "certificates.openshift.io/test-name"
	// CertificateAutoRegenerateAfterOfflineExpiryAnnotation contains a link to PR adding this annotation which verifies
	// that TLS artifact is correctly regenerated after it has expired
	CertificateAutoRegenerateAfterOfflineExpiryAnnotation string = "certificates.openshift.io/auto-regenerate-after-offline-expiry"
	// CertificateRefreshPeriodAnnotation is the interval at which the certificate should be refreshed.
	CertificateRefreshPeriodAnnotation string = "certificates.openshift.io/refresh-period"
)

type AdditionalAnnotations struct {
	// JiraComponent annotates tls artifacts so that owner could be easily found
	JiraComponent string
	// Description is a human-readable one sentence description of certificate purpose
	Description string
	// TestName is an e2e test name which verifies that TLS artifact is created and used correctly
	TestName string
	// AutoRegenerateAfterOfflineExpiry contains a link to PR which adds this annotation on the TLS artifact
	AutoRegenerateAfterOfflineExpiry string
	// NotBefore contains certificate the certificate creation date in RFC3339 format.
	NotBefore string
	// NotAfter contains certificate the certificate validity date in RFC3339 format.
	NotAfter string
	// RefreshPeriod contains the interval at which the certificate should be refreshed.
	RefreshPeriod string
}

func (a AdditionalAnnotations) EnsureTLSMetadataUpdate(meta *metav1.ObjectMeta) bool {
	modified := false
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	if len(a.JiraComponent) > 0 && meta.Annotations[annotations.OpenShiftComponent] != a.JiraComponent {
		diff := cmp.Diff(meta.Annotations[annotations.OpenShiftComponent], a.JiraComponent)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", annotations.OpenShiftComponent, meta.Namespace, meta.Name, diff)
		meta.Annotations[annotations.OpenShiftComponent] = a.JiraComponent
		modified = true
	}
	if len(a.Description) > 0 && meta.Annotations[annotations.OpenShiftDescription] != a.Description {
		diff := cmp.Diff(meta.Annotations[annotations.OpenShiftDescription], a.Description)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", annotations.OpenShiftDescription, meta.Namespace, meta.Name, diff)
		meta.Annotations[annotations.OpenShiftDescription] = a.Description
		modified = true
	}
	if len(a.TestName) > 0 && meta.Annotations[CertificateTestNameAnnotation] != a.TestName {
		diff := cmp.Diff(meta.Annotations[CertificateTestNameAnnotation], a.TestName)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", CertificateTestNameAnnotation, meta.Name, meta.Namespace, diff)
		meta.Annotations[CertificateTestNameAnnotation] = a.TestName
		modified = true
	}
	if len(a.AutoRegenerateAfterOfflineExpiry) > 0 && meta.Annotations[CertificateAutoRegenerateAfterOfflineExpiryAnnotation] != a.AutoRegenerateAfterOfflineExpiry {
		diff := cmp.Diff(meta.Annotations[CertificateAutoRegenerateAfterOfflineExpiryAnnotation], a.AutoRegenerateAfterOfflineExpiry)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", CertificateAutoRegenerateAfterOfflineExpiryAnnotation, meta.Namespace, meta.Name, diff)
		meta.Annotations[CertificateAutoRegenerateAfterOfflineExpiryAnnotation] = a.AutoRegenerateAfterOfflineExpiry
		modified = true
	}
	if len(a.NotBefore) > 0 && meta.Annotations[CertificateNotBeforeAnnotation] != a.NotBefore {
		diff := cmp.Diff(meta.Annotations[CertificateNotBeforeAnnotation], a.NotBefore)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", CertificateNotBeforeAnnotation, meta.Name, meta.Namespace, diff)
		meta.Annotations[CertificateNotBeforeAnnotation] = a.NotBefore
		modified = true
	}
	if len(a.NotAfter) > 0 && meta.Annotations[CertificateNotAfterAnnotation] != a.NotAfter {
		diff := cmp.Diff(meta.Annotations[CertificateNotAfterAnnotation], a.NotAfter)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", CertificateNotAfterAnnotation, meta.Name, meta.Namespace, diff)
		meta.Annotations[CertificateNotAfterAnnotation] = a.NotAfter
		modified = true
	}
	if len(a.RefreshPeriod) > 0 && meta.Annotations[CertificateRefreshPeriodAnnotation] != a.RefreshPeriod {
		diff := cmp.Diff(meta.Annotations[CertificateRefreshPeriodAnnotation], a.RefreshPeriod)
		klog.V(2).Infof("Updating %q annotation for %s/%s, diff: %s", CertificateRefreshPeriodAnnotation, meta.Name, meta.Namespace, diff)
		meta.Annotations[CertificateRefreshPeriodAnnotation] = a.RefreshPeriod
		modified = true
	}
	return modified
}

func NewTLSArtifactObjectMeta(name, namespace string, annotations AdditionalAnnotations) metav1.ObjectMeta {
	meta := metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
	_ = annotations.EnsureTLSMetadataUpdate(&meta)
	return meta
}
