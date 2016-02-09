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
		&OAuthAccessToken{},
		&OAuthAccessTokenList{},
		&OAuthAuthorizeToken{},
		&OAuthAuthorizeTokenList{},
		&OAuthClient{},
		&OAuthClientList{},
		&OAuthClientAuthorization{},
		&OAuthClientAuthorizationList{},
	)
}

func (obj *OAuthClientAuthorizationList) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *OAuthClientAuthorization) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *OAuthClientList) GetObjectKind() unversioned.ObjectKind              { return &obj.TypeMeta }
func (obj *OAuthClient) GetObjectKind() unversioned.ObjectKind                  { return &obj.TypeMeta }
func (obj *OAuthAuthorizeTokenList) GetObjectKind() unversioned.ObjectKind      { return &obj.TypeMeta }
func (obj *OAuthAuthorizeToken) GetObjectKind() unversioned.ObjectKind          { return &obj.TypeMeta }
func (obj *OAuthAccessTokenList) GetObjectKind() unversioned.ObjectKind         { return &obj.TypeMeta }
func (obj *OAuthAccessToken) GetObjectKind() unversioned.ObjectKind             { return &obj.TypeMeta }
