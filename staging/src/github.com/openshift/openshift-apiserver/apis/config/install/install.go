package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	clusterresourceoverrideinstall "github.com/openshift/openshift-apiserver/admission/autoscaling/clusterresourceoverride/apis/clusterresourceoverride/install"
	runoncedurationinstall "github.com/openshift/openshift-apiserver/admission/autoscaling/runonceduration/apis/runonceduration/install"
	externaliprangerinstall "github.com/openshift/openshift-apiserver/admission/externalipranger/apis/externalipranger/install"
	imagepolicyapiv1 "github.com/openshift/openshift-apiserver/admission/imagepolicy/apis/imagepolicy/v1"
	ingressadmissioninstall "github.com/openshift/openshift-apiserver/admission/ingress/apis/ingressadmission/install"
	requestlimitinstall "github.com/openshift/openshift-apiserver/admission/requestlimit/apis/requestlimit/install"
	restrictedendpointsinstall "github.com/openshift/openshift-apiserver/admission/restrictedendpoints/apis/restrictedendpoints/install"
	podnodeconstraintsinstall "github.com/openshift/openshift-apiserver/admission/scheduler/podnodeconstraints/apis/podnodeconstraints/install"
	configapi "github.com/openshift/openshift-apiserver/apis/config"
	configapiv1 "github.com/openshift/openshift-apiserver/apis/config/v1"
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
	imagepolicyapiv1.Install(scheme)

	// add the other admission config types we have
	requestlimitinstall.InstallInternal(scheme)

	// add the other admission config types we have to the core group if they are legacy types
	ingressadmissioninstall.InstallInternal(scheme)
	clusterresourceoverrideinstall.InstallInternal(scheme)
	runoncedurationinstall.InstallInternal(scheme)
	podnodeconstraintsinstall.InstallInternal(scheme)
	restrictedendpointsinstall.InstallInternal(scheme)
	externaliprangerinstall.InstallInternal(scheme)
}
