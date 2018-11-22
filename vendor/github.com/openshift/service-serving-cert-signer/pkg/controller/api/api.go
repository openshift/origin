package api

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	InjectCABundleAnnotationName = "service.alpha.openshift.io/inject-cabundle"
	InjectionDataKey             = "service-ca.crt"
)

func HasInjectCABundleAnnotation(metadata v1.Object) bool {
	return strings.EqualFold(metadata.GetAnnotations()[InjectCABundleAnnotationName], "true")
}

func HasInjectCABundleAnnotationUpdate(old, cur v1.Object) bool {
	return HasInjectCABundleAnnotation(cur)
}

// Annotations on service
const (
	// ServingCertSecretAnnotation stores the name of the secret to generate into.
	ServingCertSecretAnnotation = "service.alpha.openshift.io/serving-cert-secret-name"
	// ServingCertCreatedByAnnotation stores the of the signer common name.  This could be used later to see if the
	// services need to have the the serving certs regenerated.  The presence and matching of this annotation prevents
	// regeneration
	ServingCertCreatedByAnnotation = "service.alpha.openshift.io/serving-cert-signed-by"
	// ServingCertErrorAnnotation stores the error that caused cert generation failures.
	ServingCertErrorAnnotation = "service.alpha.openshift.io/serving-cert-generation-error"
	// ServingCertErrorNumAnnotation stores how many consecutive errors we've hit.  A value of the maxRetries will prevent
	// the controller from reattempting until it is cleared.
	ServingCertErrorNumAnnotation = "service.alpha.openshift.io/serving-cert-generation-error-num"
)

// Annotations on secret
const (
	// ServiceUIDAnnotation is an annotation on a secret that indicates which service created it, by UID
	ServiceUIDAnnotation = "service.alpha.openshift.io/originating-service-uid"
	// ServiceNameAnnotation is an annotation on a secret that indicates which service created it, by Name to allow reverse lookups on services
	// for comparison against UIDs
	ServiceNameAnnotation = "service.alpha.openshift.io/originating-service-name"
	// ServingCertExpiryAnnotation is an annotation that holds the expiry time of the certificate.  It accepts time in the
	// RFC3339 format: 2018-11-29T17:44:39Z
	ServingCertExpiryAnnotation = "service.alpha.openshift.io/expiry"
)
