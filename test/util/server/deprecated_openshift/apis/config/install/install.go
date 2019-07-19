package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
	configapiv1 "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config/v1"
)

func init() {
	InstallLegacyInternal(configapi.Scheme)
}

func InstallLegacyInternal(scheme *runtime.Scheme) {
	configapi.InstallLegacy(scheme)
	configapiv1.InstallLegacy(scheme)

	// we additionally need to enable audit versions, since we embed the audit
	// policy file inside master-config.yaml
	audit.AddToScheme(scheme)
	auditv1alpha1.AddToScheme(scheme)
	auditv1beta1.AddToScheme(scheme)
	apiserver.AddToScheme(scheme)
	apiserverv1alpha1.AddToScheme(scheme)
}
