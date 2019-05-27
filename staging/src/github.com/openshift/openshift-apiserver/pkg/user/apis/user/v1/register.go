package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	"github.com/openshift/api/user/v1"
	"github.com/openshift/origin/pkg/user/apis/user"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		user.Install,
		v1.Install,
		corev1conversions.AddToScheme,

		addFieldSelectorKeyConversions,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
