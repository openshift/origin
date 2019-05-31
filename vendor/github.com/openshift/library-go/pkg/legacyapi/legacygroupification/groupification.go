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
)

// deprecated
func IsOAPI(gvk schema.GroupVersionKind) bool {
	if len(gvk.Group) > 0 {
		return false
	}

	_, ok := oapiKindsToGroup[gvk.Kind]
	return ok
}

// deprecated
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

// deprecated
func OAPIToGroupified(uncast runtime.Object, gvk *schema.GroupVersionKind) {
	if len(gvk.Group) > 0 {
		return
	}

	switch obj := uncast.(type) {
	case *unstructured.Unstructured:
		newGroup := fixOAPIGroupKindInTopLevelUnstructured(obj.Object)
		if len(newGroup) > 0 {
			gvk.Group = newGroup
			uncast.GetObjectKind().SetGroupVersionKind(*gvk)
		}
	case *unstructured.UnstructuredList:
		newGroup := fixOAPIGroupKindInTopLevelUnstructured(obj.Object)
		if len(newGroup) > 0 {
			gvk.Group = newGroup
			uncast.GetObjectKind().SetGroupVersionKind(*gvk)
		}

	case *appsv1.DeploymentConfig, *appsv1.DeploymentConfigList,
		*appsv1.DeploymentConfigRollback,
		*appsv1.DeploymentLog,
		*appsv1.DeploymentRequest:
		gvk.Group = appsv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *authorizationv1.ClusterRoleBinding, *authorizationv1.ClusterRoleBindingList,
		*authorizationv1.ClusterRole, *authorizationv1.ClusterRoleList,
		*authorizationv1.Role, *authorizationv1.RoleList,
		*authorizationv1.RoleBinding, *authorizationv1.RoleBindingList,
		*authorizationv1.RoleBindingRestriction, *authorizationv1.RoleBindingRestrictionList,
		*authorizationv1.SubjectRulesReview, *authorizationv1.SelfSubjectRulesReview,
		*authorizationv1.ResourceAccessReview, *authorizationv1.LocalResourceAccessReview,
		*authorizationv1.SubjectAccessReview, *authorizationv1.LocalSubjectAccessReview:
		gvk.Group = authorizationv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *buildv1.BuildConfig, *buildv1.BuildConfigList,
		*buildv1.Build, *buildv1.BuildList,
		*buildv1.BuildLog,
		*buildv1.BuildRequest,
		*buildv1.BinaryBuildRequestOptions:
		gvk.Group = buildv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *imagev1.Image, *imagev1.ImageList,
		*imagev1.ImageSignature,
		*imagev1.ImageStreamImage,
		*imagev1.ImageStreamImport,
		*imagev1.ImageStreamMapping,
		*imagev1.ImageStream, *imagev1.ImageStreamList,
		*imagev1.ImageStreamTag:
		gvk.Group = imagev1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *networkv1.ClusterNetwork, *networkv1.ClusterNetworkList,
		*networkv1.NetNamespace, *networkv1.NetNamespaceList,
		*networkv1.HostSubnet, *networkv1.HostSubnetList,
		*networkv1.EgressNetworkPolicy, *networkv1.EgressNetworkPolicyList:
		gvk.Group = networkv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *projectv1.Project, *projectv1.ProjectList,
		*projectv1.ProjectRequest:
		gvk.Group = projectv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *quotav1.ClusterResourceQuota, *quotav1.ClusterResourceQuotaList,
		*quotav1.AppliedClusterResourceQuota, *quotav1.AppliedClusterResourceQuotaList:
		gvk.Group = quotav1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *oauthv1.OAuthAuthorizeToken, *oauthv1.OAuthAuthorizeTokenList,
		*oauthv1.OAuthClientAuthorization, *oauthv1.OAuthClientAuthorizationList,
		*oauthv1.OAuthClient, *oauthv1.OAuthClientList,
		*oauthv1.OAuthAccessToken, *oauthv1.OAuthAccessTokenList:
		gvk.Group = oauthv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *routev1.Route, *routev1.RouteList:
		gvk.Group = routev1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *securityv1.SecurityContextConstraints, *securityv1.SecurityContextConstraintsList,
		*securityv1.PodSecurityPolicySubjectReview,
		*securityv1.PodSecurityPolicySelfSubjectReview,
		*securityv1.PodSecurityPolicyReview:
		gvk.Group = securityv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *templatev1.Template, *templatev1.TemplateList:
		gvk.Group = templatev1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	case *userv1.Group, *userv1.GroupList,
		*userv1.Identity, *userv1.IdentityList,
		*userv1.UserIdentityMapping,
		*userv1.User, *userv1.UserList:
		gvk.Group = userv1.GroupName
		uncast.GetObjectKind().SetGroupVersionKind(*gvk)

	}
}

var oapiKindsToGroup = map[string]string{
	"DeploymentConfigRollback": "apps.openshift.io",
	"DeploymentConfig":         "apps.openshift.io", "DeploymentConfigList": "apps.openshift.io",
	"DeploymentLog":      "apps.openshift.io",
	"DeploymentRequest":  "apps.openshift.io",
	"ClusterRoleBinding": "authorization.openshift.io", "ClusterRoleBindingList": "authorization.openshift.io",
	"ClusterRole": "authorization.openshift.io", "ClusterRoleList": "authorization.openshift.io",
	"RoleBindingRestriction": "authorization.openshift.io", "RoleBindingRestrictionList": "authorization.openshift.io",
	"RoleBinding": "authorization.openshift.io", "RoleBindingList": "authorization.openshift.io",
	"Role": "authorization.openshift.io", "RoleList": "authorization.openshift.io",
	"SubjectRulesReview": "authorization.openshift.io", "SelfSubjectRulesReview": "authorization.openshift.io",
	"ResourceAccessReview": "authorization.openshift.io", "LocalResourceAccessReview": "authorization.openshift.io",
	"SubjectAccessReview": "authorization.openshift.io", "LocalSubjectAccessReview": "authorization.openshift.io",
	"BuildConfig": "build.openshift.io", "BuildConfigList": "build.openshift.io",
	"Build": "build.openshift.io", "BuildList": "build.openshift.io",
	"BinaryBuildRequestOptions": "build.openshift.io",
	"BuildLog":                  "build.openshift.io",
	"BuildRequest":              "build.openshift.io",
	"Image":                     "image.openshift.io", "ImageList": "image.openshift.io",
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
	"AppliedClusterResourceQuota": "quota.openshift.io", "AppliedClusterResourceQuotaList": "quota.openshift.io",
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
