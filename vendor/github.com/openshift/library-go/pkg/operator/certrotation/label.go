package certrotation

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// ManagedCertificateTypeLabelName marks config map or secret as object that contains managed certificates.
	// This groups all objects that store certs and allow easy query to get them all.
	// The value of this label should be set to "true".
	ManagedCertificateTypeLabelName = "auth.openshift.io/managed-certificate-type"
)

type CertificateType string

var (
	CertificateTypeCABundle CertificateType = "ca-bundle"
	CertificateTypeSigner   CertificateType = "signer"
	CertificateTypeTarget   CertificateType = "target"
	CertificateTypeUnknown  CertificateType = "unknown"
)

// LabelAsManagedConfigMap add label indicating the given config map contains certificates
// that are managed.
func LabelAsManagedConfigMap(config *v1.ConfigMap, certificateType CertificateType) {
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[ManagedCertificateTypeLabelName] = string(certificateType)
}

// LabelAsManagedConfigMap add label indicating the given secret contains certificates
// that are managed.
func LabelAsManagedSecret(secret *v1.Secret, certificateType CertificateType) {
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels[ManagedCertificateTypeLabelName] = string(certificateType)
}

// CertificateTypeFromObject returns the CertificateType based on the annotations of the object.
func CertificateTypeFromObject(obj runtime.Object) (CertificateType, error) {
	accesor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}
	actualLabels := accesor.GetLabels()
	if actualLabels == nil {
		return CertificateTypeUnknown, nil
	}

	t := CertificateType(actualLabels[ManagedCertificateTypeLabelName])
	switch t {
	case CertificateTypeCABundle, CertificateTypeSigner, CertificateTypeTarget:
		return t, nil
	default:
		return CertificateTypeUnknown, nil
	}
}
