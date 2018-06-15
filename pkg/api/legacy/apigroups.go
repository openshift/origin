package legacy

import (
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsapiv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
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

func InstallLegacyApps(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(appsapi.GroupName, appsapi.AddToSchemeInCoreGroup, appsapiv1.AddToSchemeInCoreGroup, sets.NewString(), registry, scheme)
}

func InstallLegacyAuthorization(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(authorizationapi.GroupName, authorizationapi.AddToSchemeInCoreGroup, authorizationapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterRole", "ClusterRoleBinding", "ResourceAccessReviewResponse", "SubjectAccessReviewResponse"),
		registry, scheme,
	)
}

func InstallLegacyBuild(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(buildapi.GroupName, buildapi.AddToSchemeInCoreGroup, buildapiv1.AddToSchemeInCoreGroup, sets.NewString(), registry, scheme)
}

func InstallLegacyImage(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(imageapi.GroupName, imageapi.AddToSchemeInCoreGroup, imageapiv1.AddToSchemeInCoreGroup,
		sets.NewString("Image", "ImageSignature"),
		registry, scheme,
	)
}

func InstallLegacyNetwork(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(networkapi.GroupName, networkapi.AddToSchemeInCoreGroup, networkapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterNetwork", "HostSubnet", "NetNamespace"),
		registry, scheme,
	)
}

func InstallLegacyOAuth(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(oauthapi.GroupName, oauthapi.AddToSchemeInCoreGroup, oauthapiv1.AddToSchemeInCoreGroup,
		sets.NewString("OAuthAccessToken", "OAuthAuthorizeToken", "OAuthClient", "OAuthClientAuthorization"),
		registry, scheme,
	)
}

func InstallLegacyProject(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(projectapi.GroupName, projectapi.AddToSchemeInCoreGroup, projectapiv1.AddToSchemeInCoreGroup,
		sets.NewString("Project", "ProjectRequest"),
		registry, scheme,
	)
}

func InstallLegacyQuota(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(quotaapi.GroupName, quotaapi.AddToSchemeInCoreGroup, quotaapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterResourceQuota"),
		registry, scheme,
	)
}

func InstallLegacyRoute(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(routeapi.GroupName, routeapi.AddToSchemeInCoreGroup, routeapiv1.AddToSchemeInCoreGroup, sets.NewString(), registry, scheme)
}

func InstallLegacySecurity(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(securityapi.GroupName, securityapi.AddToSchemeInCoreGroup, securityapiv1.AddToSchemeInCoreGroup,
		sets.NewString("SecurityContextConstraints"),
		registry, scheme,
	)
}

func InstallLegacyTemplate(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(templateapi.GroupName, templateapi.AddToSchemeInCoreGroup, templateapiv1.AddToSchemeInCoreGroup,
		sets.NewString("BrokerTemplateInstance"),
		registry, scheme,
	)
}

func InstallLegacyUser(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacy(userapi.GroupName, userapi.AddToSchemeInCoreGroup, userapiv1.AddToSchemeInCoreGroup,
		sets.NewString("User", "Identity", "UserIdentityMapping", "Group"),
		registry, scheme,
	)
}
