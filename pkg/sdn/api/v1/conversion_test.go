package v1_test

import (
	"testing"

	"github.com/openshift/origin/pkg/sdn/api"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "ClusterNetwork", false,
		// Ensure all currently returned labels are supported
		api.ClusterNetworkToSelectableFields(&api.ClusterNetwork{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "HostSubnet", false,
		// Ensure all currently returned labels are supported
		api.HostSubnetToSelectableFields(&api.HostSubnet{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "NetNamespace", false,
		// Ensure all currently returned labels are supported
		api.NetNamespaceToSelectableFields(&api.NetNamespace{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "EgressNetworkPolicy", true,
		// Ensure all currently returned labels are supported
		api.EgressNetworkPolicyToSelectableFields(&api.EgressNetworkPolicy{}),
	)
}
