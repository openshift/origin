package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/oauth/v1"
	"github.com/openshift/openshift-apiserver/pkg/oauth/apis/oauth"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		oauth.Install,
		v1.Install,

		addFieldSelectorKeyConversions,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
