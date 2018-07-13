package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/authorization/v1"
	"github.com/openshift/origin/pkg/authorization/apis/authorization"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		authorization.Install,
		v1.Install,
		AddConversionFuncs,
		AddFieldSelectorKeyConversions,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
