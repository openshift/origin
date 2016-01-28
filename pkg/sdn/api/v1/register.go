package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: "v1"}

func AddToScheme(scheme *runtime.Scheme) {
	addKnownTypes(scheme)
	addConversionFuncs(scheme)
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ClusterNetwork{},
		&ClusterNetworkList{},
		&HostSubnet{},
		&HostSubnetList{},
		&NetNamespace{},
		&NetNamespaceList{},
	)
}

func (obj *ClusterNetwork) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *ClusterNetworkList) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *HostSubnet) GetObjectKind() unversioned.ObjectKind         { return &obj.TypeMeta }
func (obj *HostSubnetList) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *NetNamespace) GetObjectKind() unversioned.ObjectKind       { return &obj.TypeMeta }
func (obj *NetNamespaceList) GetObjectKind() unversioned.ObjectKind   { return &obj.TypeMeta }
