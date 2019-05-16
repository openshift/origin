package api

import (
	kadmissionv1beta1 "k8s.io/api/admission/v1beta1"
	kadmissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	kappsv1 "k8s.io/api/apps/v1"
	kappsv1beta1 "k8s.io/api/apps/v1beta1"
	kappsv1beta2 "k8s.io/api/apps/v1beta2"
	kauthenticationv1 "k8s.io/api/authentication/v1"
	kauthenticationv1beta1 "k8s.io/api/authentication/v1beta1"
	kauthorizationv1 "k8s.io/api/authorization/v1"
	kauthorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	kautoscalingv1 "k8s.io/api/autoscaling/v1"
	kautoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	kbatchv1 "k8s.io/api/batch/v1"
	kbatchv1beta1 "k8s.io/api/batch/v1beta1"
	kbatchv2alpha1 "k8s.io/api/batch/v2alpha1"
	kcertificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	kcorev1 "k8s.io/api/core/v1"
	keventsv1beta1 "k8s.io/api/events/v1beta1"
	kextensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	kimagepolicyv1alpha1 "k8s.io/api/imagepolicy/v1alpha1"
	knetworkingv1 "k8s.io/api/networking/v1"
	kpolicyv1beta1 "k8s.io/api/policy/v1beta1"
	krbacv1 "k8s.io/api/rbac/v1"
	krbacv1alpha1 "k8s.io/api/rbac/v1alpha1"
	krbacv1beta1 "k8s.io/api/rbac/v1beta1"
	kschedulingv1alpha1 "k8s.io/api/scheduling/v1alpha1"
	kschedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	ksettingsv1alpha1 "k8s.io/api/settings/v1alpha1"
	kstoragev1 "k8s.io/api/storage/v1"
	kstoragev1alpha1 "k8s.io/api/storage/v1alpha1"
	kstoragev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/config"
	"github.com/openshift/api/image"
	"github.com/openshift/api/kubecontrolplane"
	"github.com/openshift/api/network"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/openshiftcontrolplane"
	"github.com/openshift/api/operator"
	"github.com/openshift/api/osin"
	"github.com/openshift/api/project"
	"github.com/openshift/api/quota"
	"github.com/openshift/api/route"
	"github.com/openshift/api/security"
	"github.com/openshift/api/servicecertsigner"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"
	"github.com/openshift/api/webconsole"

	// just make sure this compiles.  Don't add it to a scheme
	_ "github.com/openshift/api/legacyconfig/v1"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		apps.Install,
		authorization.Install,
		build.Install,
		config.Install,
		image.Install,
		kubecontrolplane.Install,
		network.Install,
		oauth.Install,
		openshiftcontrolplane.Install,
		operator.Install,
		osin.Install,
		project.Install,
		quota.Install,
		route.Install,
		security.Install,
		servicecertsigner.Install,
		template.Install,
		user.Install,
		webconsole.Install,
	)
	// Install is a function which adds every version of every openshift group to a scheme
	Install = schemeBuilder.AddToScheme

	kubeSchemeBuilder = runtime.NewSchemeBuilder(
		kadmissionv1beta1.AddToScheme,
		kadmissionregistrationv1beta1.AddToScheme,
		kappsv1.AddToScheme,
		kappsv1beta1.AddToScheme,
		kappsv1beta2.AddToScheme,
		kauthenticationv1.AddToScheme,
		kauthenticationv1beta1.AddToScheme,
		kauthorizationv1.AddToScheme,
		kauthorizationv1beta1.AddToScheme,
		kautoscalingv1.AddToScheme,
		kautoscalingv2beta1.AddToScheme,
		kbatchv1.AddToScheme,
		kbatchv1beta1.AddToScheme,
		kbatchv2alpha1.AddToScheme,
		kcertificatesv1beta1.AddToScheme,
		kcorev1.AddToScheme,
		keventsv1beta1.AddToScheme,
		kextensionsv1beta1.AddToScheme,
		kimagepolicyv1alpha1.AddToScheme,
		knetworkingv1.AddToScheme,
		kpolicyv1beta1.AddToScheme,
		krbacv1.AddToScheme,
		krbacv1beta1.AddToScheme,
		krbacv1alpha1.AddToScheme,
		kschedulingv1alpha1.AddToScheme,
		kschedulingv1beta1.AddToScheme,
		ksettingsv1alpha1.AddToScheme,
		kstoragev1.AddToScheme,
		kstoragev1beta1.AddToScheme,
		kstoragev1alpha1.AddToScheme,
	)
	// InstallKube is a way to install all the external k8s.io/api types
	InstallKube = kubeSchemeBuilder.AddToScheme
)
