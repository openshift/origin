package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/openshift-apiserver/pkg/security/apis/security"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		security.Install,
		securityv1.Install,
		corev1conversions.AddToScheme,

		AddConversionFuncs,
		AddDefaultingFuncs,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
