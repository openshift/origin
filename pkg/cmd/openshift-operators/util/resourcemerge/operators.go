package resourcemerge

import (
	webconsolev1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/webconsole/v1alpha1"
)

func EnsureWebConsoleOperatorConfig(modified *bool, existing *webconsolev1alpha1.OpenShiftWebConsoleConfig, required webconsolev1alpha1.OpenShiftWebConsoleConfig) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	if existing.Spec.ManagementState != required.Spec.ManagementState {
		*modified = true
		existing.Spec.ManagementState = required.Spec.ManagementState
	}

	SetStringIfSet(modified, &existing.Spec.ImagePullSpec, required.Spec.ImagePullSpec)
	SetStringIfSet(modified, &existing.Spec.Version, required.Spec.Version)
	SetInt64(modified, &existing.Spec.Logging.LogLevel, required.Spec.Logging.LogLevel)
}
