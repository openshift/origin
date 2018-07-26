package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/authorization/v1alpha1"
	"github.com/openshift/origin/pkg/authorization/apis/authorization"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		authorization.Install,
		v1alpha1.Install,
		RegisterConversions,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
