package v1

import (
	"testing"

	"github.com/openshift/origin/pkg/sdn/api"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "ClusterNetwork",
		// Ensure all currently returned labels are supported
		api.ClusterNetworkToSelectableFields(&api.ClusterNetwork{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "HostSubnet",
		// Ensure all currently returned labels are supported
		api.HostSubnetToSelectableFields(&api.HostSubnet{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "NetNamespace",
		// Ensure all currently returned labels are supported
		api.NetNamespaceToSelectableFields(&api.NetNamespace{}),
	)

}
