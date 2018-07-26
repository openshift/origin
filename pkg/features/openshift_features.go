package features

import (
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

const (
	// owner: @enj
	// alpha: v4.0
	//
	// Enable the experimental access restriction deny authorizer to
	// allow a layer of deny policy on top of the cluster's default RBAC
	AccessRestrictionDenyAuthorizer utilfeature.Feature = "AccessRestrictionDenyAuthorizer"
)

func init() {
	utilfeature.DefaultFeatureGate.Add(defaultOpenshiftFeatureGates)
}

// defaultOpenshiftFeatureGates consists of all known Openshift-specific feature keys.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout Openshift binaries.
var defaultOpenshiftFeatureGates = map[utilfeature.Feature]utilfeature.FeatureSpec{
	AccessRestrictionDenyAuthorizer: {Default: false, PreRelease: utilfeature.Alpha},
}
