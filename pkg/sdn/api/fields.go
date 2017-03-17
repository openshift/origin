package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// ClusterNetworkToSelectableFields returns a label set that represents the object
func ClusterNetworkToSelectableFields(network *ClusterNetwork) fields.Set {
	return generic.ObjectMetaFieldsSet(&network.ObjectMeta, false)
}

// HostSubnetToSelectableFields returns a label set that represents the object
func HostSubnetToSelectableFields(obj *HostSubnet) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

// NetNamespaceToSelectableFields returns a label set that represents the object
func NetNamespaceToSelectableFields(obj *NetNamespace) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

// EgressNetworkPolicyToSelectableFields returns a label set that represents the object
func EgressNetworkPolicyToSelectableFields(obj *EgressNetworkPolicy) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
}
