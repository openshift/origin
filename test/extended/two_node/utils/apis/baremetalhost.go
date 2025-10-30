// Package apis provides BareMetalHost utilities: status checks, provisioning state monitoring, and Metal3 operations.
package apis

import (
	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
)

// GetBMHProvisioningState retrieves the current provisioning state of a BareMetalHost.
//
//	state, err := GetBMHProvisioningState(oc, "master-0", "openshift-machine-api")
func GetBMHProvisioningState(oc *exutil.CLI, bmhName, namespace string) (metal3v1alpha1.ProvisioningState, error) {
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return "", core.WrapError("get BareMetalHost", bmhName, err)
	}

	var bmh metal3v1alpha1.BareMetalHost
	if err := utils.DecodeObject(bmhOutput, &bmh); err != nil {
		return "", core.WrapError("decode BareMetalHost YAML", bmhName, err)
	}

	return bmh.Status.Provisioning.State, nil
}

// GetBMHErrorMessage retrieves the error message from a BareMetalHost's status.
//
//	errorMsg, err := GetBMHErrorMessage(oc, "master-0", "openshift-machine-api")
func GetBMHErrorMessage(oc *exutil.CLI, bmhName, namespace string) (string, error) {
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return "", core.WrapError("get BareMetalHost", bmhName, err)
	}

	var bmh metal3v1alpha1.BareMetalHost
	if err := utils.DecodeObject(bmhOutput, &bmh); err != nil {
		return "", core.WrapError("decode BareMetalHost YAML", bmhName, err)
	}

	return bmh.Status.ErrorMessage, nil
}

// GetBMH retrieves and parses a BareMetalHost resource.
//
//	bmh, err := GetBMH(oc, "master-0", "openshift-machine-api")
func GetBMH(oc *exutil.CLI, bmhName, namespace string) (*metal3v1alpha1.BareMetalHost, error) {
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return nil, core.WrapError("get BareMetalHost", bmhName, err)
	}

	var bmh metal3v1alpha1.BareMetalHost
	if err := utils.DecodeObject(bmhOutput, &bmh); err != nil {
		return nil, core.WrapError("decode BareMetalHost YAML", bmhName, err)
	}

	return &bmh, nil
}
