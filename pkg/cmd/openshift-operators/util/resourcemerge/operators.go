package resourcemerge

import (
	apiserverv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	controllerv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
)

func EnsureAPIServerOperatorConfig(modified *bool, existing *apiserverv1.OpenShiftAPIServerConfig, required apiserverv1.OpenShiftAPIServerConfig) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	SetStringIfSet(modified, &existing.Spec.ImagePullSpec, required.Spec.ImagePullSpec)
	SetStringIfSet(modified, &existing.Spec.Version, required.Spec.Version)
	SetInt64(modified, &existing.Spec.APIServerConfig.LogLevel, required.Spec.APIServerConfig.LogLevel)
	SetStringIfSet(modified, &existing.Spec.APIServerConfig.HostPath, required.Spec.APIServerConfig.HostPath)
}

func EnsureControllerOperatorConfig(modified *bool, existing *controllerv1.OpenShiftControllerConfig, required controllerv1.OpenShiftControllerConfig) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	SetStringIfSet(modified, &existing.Spec.ImagePullSpec, required.Spec.ImagePullSpec)
	SetStringIfSet(modified, &existing.Spec.Version, required.Spec.Version)
	SetInt64(modified, &existing.Spec.ControllerConfig.LogLevel, required.Spec.ControllerConfig.LogLevel)
	SetStringIfSet(modified, &existing.Spec.ControllerConfig.HostPath, required.Spec.ControllerConfig.HostPath)
}

func EnsureWebConsoleOperatorConfig(modified *bool, existing *webconsolev1.OpenShiftWebConsoleConfig, required webconsolev1.OpenShiftWebConsoleConfig) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	SetStringIfSet(modified, &existing.Spec.ImagePullSpec, required.Spec.ImagePullSpec)
	SetStringIfSet(modified, &existing.Spec.Version, required.Spec.Version)
	SetInt64(modified, &existing.Spec.LogLevel, required.Spec.LogLevel)
}
