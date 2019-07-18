package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	networkv1 "github.com/openshift/api/network/v1"
)

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
