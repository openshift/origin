package api

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	_ "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization"
	_ "github.com/openshift/origin/pkg/build/apis/build"
	_ "github.com/openshift/origin/pkg/image/apis/image"
	_ "github.com/openshift/origin/pkg/network/apis/network"
	_ "github.com/openshift/origin/pkg/oauth/apis/oauth"
	_ "github.com/openshift/origin/pkg/project/apis/project"
	_ "github.com/openshift/origin/pkg/route/apis/route"
	_ "github.com/openshift/origin/pkg/security/apis/security"
	_ "github.com/openshift/origin/pkg/template/apis/template"
	_ "github.com/openshift/origin/pkg/user/apis/user"
)

const (
	Prefix    = "/oapi"
	GroupName = ""
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
