package certrotation

import (
	"github.com/openshift/api/annotations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AutoRegenerateAfterOfflineExpiryAnnotation string = "certificates.openshift.io/auto-regenerate-after-offline-expiry"
)

type AdditionalAnnotations struct {
	// JiraComponent annotates tls artifacts so that owner could be easily found
	JiraComponent string
	// Description is a human-readable one sentence description of certificate purpose
	Description string
	// AutoRegenerateAfterOfflineExpiry contains a link to PR and an e2e test name which verifies
	// that TLS artifact is correctly regenerated after it has expired
	AutoRegenerateAfterOfflineExpiry string
}

func (a AdditionalAnnotations) EnsureTLSMetadataUpdate(meta *metav1.ObjectMeta) bool {
	modified := false
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	if len(a.JiraComponent) > 0 && meta.Annotations[annotations.OpenShiftComponent] != a.JiraComponent {
		meta.Annotations[annotations.OpenShiftComponent] = a.JiraComponent
		modified = true
	}
	if len(a.Description) > 0 && meta.Annotations[annotations.OpenShiftDescription] != a.Description {
		meta.Annotations[annotations.OpenShiftDescription] = a.Description
		modified = true
	}
	if len(a.AutoRegenerateAfterOfflineExpiry) > 0 && meta.Annotations[AutoRegenerateAfterOfflineExpiryAnnotation] != a.AutoRegenerateAfterOfflineExpiry {
		meta.Annotations[AutoRegenerateAfterOfflineExpiryAnnotation] = a.AutoRegenerateAfterOfflineExpiry
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
