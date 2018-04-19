package webconsole_operator

import (
	operatorsv1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/operators/v1alpha1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/apis/operators/v1alpha1helpers"
	webconsolev1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/webconsole/v1alpha1"
)

// most of the time the sync method will be good for a large span of minor versions
func sync_v310_00_to_00(c WebConsoleOperator, operatorConfig *webconsolev1alpha1.OpenShiftWebConsoleConfig) (operatorsv1alpha1.VersionAvailablity, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	// TODO do some work

	v1alpha1helpers.SetErrors(&versionAvailability, errors...)

	return versionAvailability, errors
}
