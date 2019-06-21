package install

import (
	crdinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	kcomponentconfiginstall "k8s.io/kubernetes/cmd/cloud-controller-manager/app/apis/config/scheme"
	kadmissioninstall "k8s.io/kubernetes/pkg/apis/admission/install"
	kadmissionregistrationinstall "k8s.io/kubernetes/pkg/apis/admissionregistration/install"
	kappsinstall "k8s.io/kubernetes/pkg/apis/apps/install"
	kauthenticationinstall "k8s.io/kubernetes/pkg/apis/authentication/install"
	kauthorizationinstall "k8s.io/kubernetes/pkg/apis/authorization/install"
	kautoscalinginstall "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	kbatchinstall "k8s.io/kubernetes/pkg/apis/batch/install"
	kcertificatesinstall "k8s.io/kubernetes/pkg/apis/certificates/install"
	kcoreinstall "k8s.io/kubernetes/pkg/apis/core/install"
	keventsinstall "k8s.io/kubernetes/pkg/apis/events/install"
	kextensionsinstall "k8s.io/kubernetes/pkg/apis/extensions/install"
	kimagepolicyinstall "k8s.io/kubernetes/pkg/apis/imagepolicy/install"
	knetworkinginstall "k8s.io/kubernetes/pkg/apis/networking/install"
	kpolicyinstall "k8s.io/kubernetes/pkg/apis/policy/install"
	krbacinstall "k8s.io/kubernetes/pkg/apis/rbac/install"
	kschedulinginstall "k8s.io/kubernetes/pkg/apis/scheduling/install"
	ksettingsinstall "k8s.io/kubernetes/pkg/apis/settings/install"
	kstorageinstall "k8s.io/kubernetes/pkg/apis/storage/install"

	oapps "github.com/openshift/openshift-apiserver/pkg/apps/apis/apps/install"
	authz "github.com/openshift/openshift-apiserver/pkg/authorization/apis/authorization/install"
	build "github.com/openshift/openshift-apiserver/pkg/build/apis/build/install"
	image "github.com/openshift/openshift-apiserver/pkg/image/apis/image/install"
	oauth "github.com/openshift/openshift-apiserver/pkg/oauth/apis/oauth/install"
	project "github.com/openshift/openshift-apiserver/pkg/project/apis/project/install"
	quota "github.com/openshift/openshift-apiserver/pkg/quota/apis/quota/install"
	route "github.com/openshift/openshift-apiserver/pkg/route/apis/route/install"
	security "github.com/openshift/openshift-apiserver/pkg/security/apis/security/install"
	template "github.com/openshift/openshift-apiserver/pkg/template/apis/template/install"
	user "github.com/openshift/openshift-apiserver/pkg/user/apis/user/install"
)

func InstallInternalOpenShift(scheme *runtime.Scheme) {
	oapps.Install(scheme)
	authz.Install(scheme)
	build.Install(scheme)
	image.Install(scheme)
	oauth.Install(scheme)
	project.Install(scheme)
	quota.Install(scheme)
	route.Install(scheme)
	security.Install(scheme)
	template.Install(scheme)
	user.Install(scheme)
}

func InstallInternalKube(scheme *runtime.Scheme) {
	crdinstall.Install(scheme)

	apiregistrationinstall.Install(scheme)

	kadmissioninstall.Install(scheme)
	kadmissionregistrationinstall.Install(scheme)
	kappsinstall.Install(scheme)
	kauthenticationinstall.Install(scheme)
	kauthorizationinstall.Install(scheme)
	kautoscalinginstall.Install(scheme)
	kbatchinstall.Install(scheme)
	kcertificatesinstall.Install(scheme)
	kcomponentconfiginstall.AddToScheme(scheme)
	kcoreinstall.Install(scheme)
	keventsinstall.Install(scheme)
	kextensionsinstall.Install(scheme)
	kimagepolicyinstall.Install(scheme)
	knetworkinginstall.Install(scheme)
	kpolicyinstall.Install(scheme)
	krbacinstall.Install(scheme)
	kschedulinginstall.Install(scheme)
	ksettingsinstall.Install(scheme)
	kstorageinstall.Install(scheme)
}
