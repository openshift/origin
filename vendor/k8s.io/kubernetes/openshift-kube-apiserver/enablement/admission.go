package enablement

import (
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation/customresourcevalidationregistration"
	"k8s.io/kubernetes/openshift-kube-apiserver/kubeadmission"
)

func InstallOpenShiftAdmissionPlugins(o *options.ServerRunOptions) {
	existingAdmissionOrder := o.Admission.GenericAdmission.RecommendedPluginOrder
	o.Admission.GenericAdmission.RecommendedPluginOrder = kubeadmission.NewOrderedKubeAdmissionPlugins(existingAdmissionOrder)
	kubeadmission.RegisterOpenshiftKubeAdmissionPlugins(o.Admission.GenericAdmission.Plugins)
	customresourcevalidationregistration.RegisterCustomResourceValidation(o.Admission.GenericAdmission.Plugins)
	existingDefaultOff := o.Admission.GenericAdmission.DefaultOffPlugins
	o.Admission.GenericAdmission.DefaultOffPlugins = kubeadmission.NewDefaultOffPluginsFunc(existingDefaultOff)()
}
