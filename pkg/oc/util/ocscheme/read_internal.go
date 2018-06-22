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
	"github.com/openshift/origin/pkg/api/legacy"
)

// ReadingInternalScheme contains:
// 1. internal upstream and downstream types
// 2. external groupified
// 3. external non-groupified
var ReadingInternalScheme = runtime.NewScheme()

func init() {
	install.InstallAll(ReadingInternalScheme)
	legacy.LegacyInstallAll(ReadingInternalScheme)

	kadmissioninstall.Install(ReadingInternalScheme)
	kadmissionregistrationinstall.Install(ReadingInternalScheme)
	kappsinstall.Install(ReadingInternalScheme)
	kauthenticationinstall.Install(ReadingInternalScheme)
	kauthorizationinstall.Install(ReadingInternalScheme)
	kautoscalinginstall.Install(ReadingInternalScheme)
	kbatchinstall.Install(ReadingInternalScheme)
	kcertificatesinstall.Install(ReadingInternalScheme)
	kcomponentconfiginstall.Install(ReadingInternalScheme)
	kcoreinstall.Install(ReadingInternalScheme)
	keventsinstall.Install(ReadingInternalScheme)
	kextensionsinstall.Install(ReadingInternalScheme)
	kimagepolicyinstall.Install(ReadingInternalScheme)
	knetworkinginstall.Install(ReadingInternalScheme)
	kpolicyinstall.Install(ReadingInternalScheme)
	krbacinstall.Install(ReadingInternalScheme)
	kschedulinginstall.Install(ReadingInternalScheme)
	ksettingsinstall.Install(ReadingInternalScheme)
	kstorageinstall.Install(ReadingInternalScheme)
}
