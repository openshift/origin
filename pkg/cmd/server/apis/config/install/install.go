package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	defaultsinstall "github.com/openshift/origin/pkg/build/controller/build/apis/defaults/install"
	overridesinstall "github.com/openshift/origin/pkg/build/controller/build/apis/overrides/install"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	imagepolicyinstall "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy/install"
	ingressadmissioninstall "github.com/openshift/origin/pkg/network/apiserver/admission/apis/ingressadmission/install"
	requestlimitinstall "github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/install"
	clusterresourceoverrideinstall "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride/install"
	runoncedurationinstall "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/runonceduration/install"
	podnodeconstraintsinstall "github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/install"
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

	// add the other admission config types we have
	requestlimitinstall.InstallInternal(scheme)

	// add the other admission config types we have to the core group if they are legacy types
	defaultsinstall.InstallLegacyInternal(scheme)
	overridesinstall.InstallLegacyInternal(scheme)
	imagepolicyinstall.InstallLegacyInternal(scheme)
	ingressadmissioninstall.InstallLegacyInternal(scheme)
	requestlimitinstall.InstallLegacyInternal(scheme)
	clusterresourceoverrideinstall.InstallLegacyInternal(scheme)
	runoncedurationinstall.InstallLegacyInternal(scheme)
	podnodeconstraintsinstall.InstallLegacyInternal(scheme)
}
