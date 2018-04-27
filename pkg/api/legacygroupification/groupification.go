package legacygroupification

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "github.com/openshift/api/apps/v1"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	networkv1 "github.com/openshift/api/network/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/project/apis/project"
	"github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/security/apis/security"
	"github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/user/apis/user"
)

func OAPIToGroupifiedGVK(gvk *schema.GroupVersionKind) {
	if len(gvk.Group) > 0 {
		return
	}

	newGroup, ok := oapiKindsToGroup[gvk.Kind]
	if !ok {
		return
	}
	gvk.Group = newGroup
}

func OAPIToGroupified(uncast runtime.Object, gvk *schema.GroupVersionKind) {
	if len(gvk.Group) > 0 {
		return
	}

	switch obj := uncast.(type) {
	case *unstructured.Unstructured:
		newGroup := fixOAPIGroupKindInTopLevelUnstructured(obj.Object)
		if len(newGroup) > 0 {
			gvk.Group = newGroup
		}
	case *unstructured.UnstructuredList:
		newGroup := fixOAPIGroupKindInTopLevelUnstructured(obj.Object)
		if len(newGroup) > 0 {
			gvk.Group = newGroup
		}

	case *apps.DeploymentConfig, *appsv1.DeploymentConfig, *apps.DeploymentConfigList, *appsv1.DeploymentConfigList,
		*apps.DeploymentConfigRollback, *appsv1.DeploymentConfigRollback:
		gvk.Group = apps.GroupName

	case *authorization.ClusterRoleBinding, *authorizationv1.ClusterRoleBinding, *authorization.ClusterRoleBindingList, *authorizationv1.ClusterRoleBindingList,
		*authorization.ClusterRole, *authorizationv1.ClusterRole, *authorization.ClusterRoleList, *authorizationv1.ClusterRoleList,
		*authorization.Role, *authorizationv1.Role, *authorization.RoleList, *authorizationv1.RoleList,
		*authorization.RoleBinding, *authorizationv1.RoleBinding, *authorization.RoleBindingList, *authorizationv1.RoleBindingList,
		*authorization.RoleBindingRestriction, *authorizationv1.RoleBindingRestriction, *authorization.RoleBindingRestrictionList, *authorizationv1.RoleBindingRestrictionList:
		gvk.Group = authorization.GroupName

	case *build.BuildConfig, *buildv1.BuildConfig, *build.BuildConfigList, *buildv1.BuildConfigList,
		*build.Build, *buildv1.Build, *build.BuildList, *buildv1.BuildList:
		gvk.Group = build.GroupName

	case *image.Image, *imagev1.Image, *image.ImageList, *imagev1.ImageList,
		*image.ImageSignature, *imagev1.ImageSignature,
		*image.ImageStreamImage, *imagev1.ImageStreamImage,
		*image.ImageStreamImport, *imagev1.ImageStreamImport,
		*image.ImageStreamMapping, *imagev1.ImageStreamMapping,
		*image.ImageStream, *imagev1.ImageStream, *image.ImageStreamList, *imagev1.ImageStreamList,
		*image.ImageStreamTag, *imagev1.ImageStreamTag:
		gvk.Group = image.GroupName

	case *network.ClusterNetwork, *networkv1.ClusterNetwork, *network.ClusterNetworkList, *networkv1.ClusterNetworkList,
		*network.NetNamespace, *networkv1.NetNamespace, *network.NetNamespaceList, *networkv1.NetNamespaceList,
		*network.HostSubnet, *networkv1.HostSubnet, *network.HostSubnetList, *networkv1.HostSubnetList,
		*network.EgressNetworkPolicy, *networkv1.EgressNetworkPolicy, *network.EgressNetworkPolicyList, *networkv1.EgressNetworkPolicyList:
		gvk.Group = network.GroupName

	case *project.Project, *projectv1.Project, *project.ProjectList, *projectv1.ProjectList,
		*project.ProjectRequest, *projectv1.ProjectRequest:
		gvk.Group = project.GroupName

	case *quota.ClusterResourceQuota, *quotav1.ClusterResourceQuota, *quota.ClusterResourceQuotaList, *quotav1.ClusterResourceQuotaList:
		gvk.Group = quota.GroupName

	case *oauth.OAuthAuthorizeToken, *oauthv1.OAuthAuthorizeToken, *oauth.OAuthAuthorizeTokenList, *oauthv1.OAuthAuthorizeTokenList,
		*oauth.OAuthClientAuthorization, *oauthv1.OAuthClientAuthorization, *oauth.OAuthClientAuthorizationList, *oauthv1.OAuthClientAuthorizationList,
		*oauth.OAuthClient, *oauthv1.OAuthClient, *oauth.OAuthClientList, *oauthv1.OAuthClientList,
		*oauth.OAuthAccessToken, *oauthv1.OAuthAccessToken, *oauth.OAuthAccessTokenList, *oauthv1.OAuthAccessTokenList:
		gvk.Group = oauth.GroupName

	case *route.Route, *routev1.Route, *route.RouteList, *routev1.RouteList:
		gvk.Group = route.GroupName

	case *security.SecurityContextConstraints, *securityv1.SecurityContextConstraints, *security.SecurityContextConstraintsList, *securityv1.SecurityContextConstraintsList,
		*security.PodSecurityPolicySubjectReview, *securityv1.PodSecurityPolicySubjectReview,
		*security.PodSecurityPolicySelfSubjectReview, *securityv1.PodSecurityPolicySelfSubjectReview,
		*security.PodSecurityPolicyReview, *securityv1.PodSecurityPolicyReview:
		gvk.Group = security.GroupName

	case *template.Template, *templatev1.Template, *template.TemplateList, *templatev1.TemplateList:
		gvk.Group = template.GroupName

	case *user.Group, *userv1.Group, *user.GroupList, *userv1.GroupList,
		*user.Identity, *userv1.Identity, *user.IdentityList, *userv1.IdentityList,
		*user.UserIdentityMapping, *userv1.UserIdentityMapping,
		*user.User, *userv1.User, *user.UserList, *userv1.UserList:
		gvk.Group = user.GroupName

	}
}

var oapiKindsToGroup = map[string]string{
	"DeploymentConfigRollback": "apps.openshift.io",
	"DeploymentConfig":         "apps.openshift.io", "DeploymentConfigList": "apps.openshift.io",
	"ClusterRoleBinding": "authorization.openshift.io", "ClusterRoleBindingList": "authorization.openshift.io",
	"ClusterRole": "authorization.openshift.io", "ClusterRoleList": "authorization.openshift.io",
	"RoleBindingRestriction": "authorization.openshift.io", "RoleBindingRestrictionList": "authorization.openshift.io",
	"RoleBinding": "authorization.openshift.io", "RoleBindingList": "authorization.openshift.io",
	"Role": "authorization.openshift.io", "RoleList": "authorization.openshift.io",
	"BuildConfig": "build.openshift.io", "BuildConfigList": "build.openshift.io",
	"Build": "build.openshift.io", "BuildList": "build.openshift.io",
	"Image": "image.openshift.io", "ImageList": "image.openshift.io",
	"ImageSignature":     "image.openshift.io",
	"ImageStreamImage":   "image.openshift.io",
	"ImageStreamImport":  "image.openshift.io",
	"ImageStreamMapping": "image.openshift.io",
	"ImageStream":        "image.openshift.io", "ImageStreamList": "image.openshift.io",
	"ImageStreamTag": "image.openshift.io", "ImageStreamTagList": "image.openshift.io",
	"ClusterNetwork": "network.openshift.io", "ClusterNetworkList": "network.openshift.io",
	"EgressNetworkPolicy": "network.openshift.io", "EgressNetworkPolicyList": "network.openshift.io",
	"HostSubnet": "network.openshift.io", "HostSubnetList": "network.openshift.io",
	"NetNamespace": "network.openshift.io", "NetNamespaceList": "network.openshift.io",
	"OAuthAccessToken": "oauth.openshift.io", "OAuthAccessTokenList": "oauth.openshift.io",
	"OAuthAuthorizeToken": "oauth.openshift.io", "OAuthAuthorizeTokenList": "oauth.openshift.io",
	"OAuthClientAuthorization": "oauth.openshift.io", "OAuthClientAuthorizationList": "oauth.openshift.io",
	"OAuthClient": "oauth.openshift.io", "OAuthClientList": "oauth.openshift.io",
	"Project": "project.openshift.io", "ProjectList": "project.openshift.io",
	"ProjectRequest":       "project.openshift.io",
	"ClusterResourceQuota": "quota.openshift.io", "ClusterResourceQuotaList": "quota.openshift.io",
	"Route": "route.openshift.io", "RouteList": "route.openshift.io",
	"SecurityContextConstraints": "security.openshift.io", "SecurityContextConstraintsList": "security.openshift.io",
	"PodSecurityPolicySubjectReview":     "security.openshift.io",
	"PodSecurityPolicySelfSubjectReview": "security.openshift.io",
	"PodSecurityPolicyReview":            "security.openshift.io",
	"Template":                           "template.openshift.io", "TemplateList": "template.openshift.io",
	"Group": "user.openshift.io", "GroupList": "user.openshift.io",
	"Identity": "user.openshift.io", "IdentityList": "user.openshift.io",
	"UserIdentityMapping": "user.openshift.io",
	"User":                "user.openshift.io", "UserList": "user.openshift.io",
}

func fixOAPIGroupKindInTopLevelUnstructured(obj map[string]interface{}) string {
	kind, ok := obj["kind"]
	if !ok {
		return ""
	}
	kindStr, ok := kind.(string)
	if !ok {
		return ""
	}
	newGroup, ok := oapiKindsToGroup[kindStr]
	if !ok {
		return ""
	}

	apiVersion, ok := obj["apiVersion"]
	if !ok {
		return newGroup
	}
	apiVersionStr, ok := apiVersion.(string)
	if !ok {
		return newGroup
	}

	if apiVersionStr != "v1" {
		return newGroup
	}
	obj["apiVersion"] = newGroup + "/v1"

	return newGroup
}
