package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: "v1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&User{},
		&UserList{},
		&Identity{},
		&IdentityList{},
		&UserIdentityMapping{},
		&Group{},
		&GroupList{},
	)
	return nil
}

func (obj *GroupList) GetObjectKind() unversioned.ObjectKind           { return &obj.TypeMeta }
func (obj *Group) GetObjectKind() unversioned.ObjectKind               { return &obj.TypeMeta }
func (obj *User) GetObjectKind() unversioned.ObjectKind                { return &obj.TypeMeta }
func (obj *UserList) GetObjectKind() unversioned.ObjectKind            { return &obj.TypeMeta }
func (obj *Identity) GetObjectKind() unversioned.ObjectKind            { return &obj.TypeMeta }
func (obj *IdentityList) GetObjectKind() unversioned.ObjectKind        { return &obj.TypeMeta }
func (obj *UserIdentityMapping) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
