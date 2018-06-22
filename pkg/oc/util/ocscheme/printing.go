package ocscheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	kadmissioninstall "k8s.io/kubernetes/pkg/apis/admission/install"
	kadmissionregistrationinstall "k8s.io/kubernetes/pkg/apis/admissionregistration/install"
	kappsinstall "k8s.io/kubernetes/pkg/apis/apps/install"
	kauthenticationinstall "k8s.io/kubernetes/pkg/apis/authentication/install"
	kauthorizationinstall "k8s.io/kubernetes/pkg/apis/authorization/install"
	kautoscalinginstall "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	kbatchinstall "k8s.io/kubernetes/pkg/apis/batch/install"
	kcertificatesinstall "k8s.io/kubernetes/pkg/apis/certificates/install"
	kcomponentconfiginstall "k8s.io/kubernetes/pkg/apis/componentconfig/install"
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

	"github.com/openshift/origin/pkg/api/install"
)

// PrintingInternalScheme contains:
// 1. internal upstream and downstream types
// 2. external groupified
var PrintingInternalScheme = runtime.NewScheme()

func init() {
	install.InstallAll(PrintingInternalScheme)

	kadmissioninstall.Install(PrintingInternalScheme)
	kadmissionregistrationinstall.Install(PrintingInternalScheme)
	kappsinstall.Install(PrintingInternalScheme)
	kauthenticationinstall.Install(PrintingInternalScheme)
	kauthorizationinstall.Install(PrintingInternalScheme)
	kautoscalinginstall.Install(PrintingInternalScheme)
	kbatchinstall.Install(PrintingInternalScheme)
	kcertificatesinstall.Install(PrintingInternalScheme)
	kcomponentconfiginstall.Install(PrintingInternalScheme)
	kcoreinstall.Install(PrintingInternalScheme)
	keventsinstall.Install(PrintingInternalScheme)
	kextensionsinstall.Install(PrintingInternalScheme)
	kimagepolicyinstall.Install(PrintingInternalScheme)
	knetworkinginstall.Install(PrintingInternalScheme)
	kpolicyinstall.Install(PrintingInternalScheme)
	krbacinstall.Install(PrintingInternalScheme)
	kschedulinginstall.Install(PrintingInternalScheme)
	ksettingsinstall.Install(PrintingInternalScheme)
	kstorageinstall.Install(PrintingInternalScheme)
}
