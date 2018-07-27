package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	networkv1 "github.com/openshift/api/network/v1"
	"github.com/openshift/origin/pkg/network/apis/network"
	networkv1helpers "github.com/openshift/origin/pkg/network/apis/network/v1"
)

// InstallLegacyNetwork this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacyNetwork(scheme *runtime.Scheme) {
	InstallExternalLegacyNetwork(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalNetworkTypes,

		networkv1helpers.RegisterDefaults,
		networkv1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func InstallExternalLegacyNetwork(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedNetworkTypes,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedNetworkTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&networkv1.ClusterNetwork{},
		&networkv1.ClusterNetworkList{},
		&networkv1.HostSubnet{},
		&networkv1.HostSubnetList{},
		&networkv1.NetNamespace{},
		&networkv1.NetNamespaceList{},
		&networkv1.EgressNetworkPolicy{},
		&networkv1.EgressNetworkPolicyList{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}

func addUngroupifiedInternalNetworkTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalGroupVersion,
		&network.ClusterNetwork{},
		&network.ClusterNetworkList{},
		&network.HostSubnet{},
		&network.HostSubnetList{},
		&network.NetNamespace{},
		&network.NetNamespaceList{},
		&network.EgressNetworkPolicy{},
		&network.EgressNetworkPolicyList{},
	)
	return nil
}
