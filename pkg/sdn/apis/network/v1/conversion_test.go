package v1_test

import (
	"testing"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "network.openshift.io/v1", "ClusterNetwork",
		// Ensure all currently returned labels are supported
		sdnapi.ClusterNetworkToSelectableFields(&sdnapi.ClusterNetwork{}),
	)

	testutil.CheckFieldLabelConversions(t, "network.openshift.io/v1", "HostSubnet",
		// Ensure all currently returned labels are supported
		sdnapi.HostSubnetToSelectableFields(&sdnapi.HostSubnet{}),
	)

	testutil.CheckFieldLabelConversions(t, "network.openshift.io/v1", "NetNamespace",
		// Ensure all currently returned labels are supported
		sdnapi.NetNamespaceToSelectableFields(&sdnapi.NetNamespace{}),
	)

	testutil.CheckFieldLabelConversions(t, "network.openshift.io/v1", "EgressNetworkPolicy",
		// Ensure all currently returned labels are supported
		sdnapi.EgressNetworkPolicyToSelectableFields(&sdnapi.EgressNetworkPolicy{}),
	)
}

func TestLegacyFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "ClusterNetwork",
		// Ensure all currently returned labels are supported
		sdnapi.ClusterNetworkToSelectableFields(&sdnapi.ClusterNetwork{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "HostSubnet",
		// Ensure all currently returned labels are supported
		sdnapi.HostSubnetToSelectableFields(&sdnapi.HostSubnet{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "NetNamespace",
		// Ensure all currently returned labels are supported
		sdnapi.NetNamespaceToSelectableFields(&sdnapi.NetNamespace{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "EgressNetworkPolicy",
		// Ensure all currently returned labels are supported
		sdnapi.EgressNetworkPolicyToSelectableFields(&sdnapi.EgressNetworkPolicy{}),
	)
}
