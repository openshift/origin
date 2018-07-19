package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	"github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/security/apis/security"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		security.Install,
		v1.Install,
		corev1conversions.AddToScheme,

		AddConversionFuncs,
		AddDefaultingFuncs,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
