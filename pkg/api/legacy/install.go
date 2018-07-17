package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	networkapiv1 "github.com/openshift/origin/pkg/network/apis/network/v1"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectapiv1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	quotaapiv1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userapiv1 "github.com/openshift/origin/pkg/user/apis/user/v1"
)

var (
	GroupName            = ""
	GroupVersion         = schema.GroupVersion{Group: GroupName, Version: "v1"}
	internalGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
)

// DEPRECATED
func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}

// DEPRECATED
func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

func InstallLegacyBuild(scheme *runtime.Scheme) {
	utilruntime.Must(buildapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(buildapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyImage(scheme *runtime.Scheme) {
	utilruntime.Must(imageapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(imageapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyNetwork(scheme *runtime.Scheme) {
	utilruntime.Must(networkapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(networkapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyOAuth(scheme *runtime.Scheme) {
	utilruntime.Must(oauthapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(oauthapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyProject(scheme *runtime.Scheme) {
	utilruntime.Must(projectapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(projectapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyQuota(scheme *runtime.Scheme) {
	utilruntime.Must(quotaapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(quotaapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyRoute(scheme *runtime.Scheme) {
	utilruntime.Must(routeapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(routeapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacySecurity(scheme *runtime.Scheme) {
	utilruntime.Must(securityapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(securityapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyTemplate(scheme *runtime.Scheme) {
	utilruntime.Must(templateapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(templateapiv1.AddToSchemeInCoreGroup(scheme))
}

func InstallLegacyUser(scheme *runtime.Scheme) {
	utilruntime.Must(userapi.AddToSchemeInCoreGroup(scheme))
	utilruntime.Must(userapiv1.AddToSchemeInCoreGroup(scheme))
}

func LegacyInstallAll(scheme *runtime.Scheme) {
	InstallLegacyApps(scheme)
	InstallLegacyAuthorization(scheme)
	InstallLegacyBuild(scheme)
	InstallLegacyImage(scheme)
	InstallLegacyNetwork(scheme)
	InstallLegacyOAuth(scheme)
	InstallLegacyProject(scheme)
	InstallLegacyQuota(scheme)
	InstallLegacyRoute(scheme)
	InstallLegacySecurity(scheme)
	InstallLegacyTemplate(scheme)
	InstallLegacyUser(scheme)
}
