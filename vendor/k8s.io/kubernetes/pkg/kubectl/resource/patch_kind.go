package resource

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var oapiKindsToGroup = map[string]string{
	"BuildConfig":               "build.openshift.io",
	"Build":                     "build.openshift.io",
	"ClusterNetwork":            "network.openshift.io",
	"ClusterResourceQuota":      "quota.openshift.io",
	"ClusterRoleBinding":        "authorization.openshift.io",
	"ClusterRole":               "authorization.openshift.io",
	"DeploymentConfigRollback":  "apps.openshift.io",
	"DeploymentConfig":          "apps.openshift.io",
	"EgressNetworkPolicy":       "network.openshift.io",
	"Group":                     "user.openshift.io",
	"HostSubnet":                "network.openshift.io",
	"Identity":                  "user.openshift.io",
	"Image":                     "image.openshift.io",
	"ImageSignature":            "image.openshift.io",
	"ImageStreamImage":          "image.openshift.io",
	"ImageStreamImport":         "image.openshift.io",
	"ImageStreamMapping":        "image.openshift.io",
	"ImageStream":               "image.openshift.io",
	"ImageStreamTag":            "image.openshift.io",
	"NetNamespace":              "network.openshift.io",
	"OAuthAccessToken":          "oauth.openshift.io",
	"OAuthAuthorizeToken":       "oauth.openshift.io",
	"OAuthClientAuthorization":  "oauth.openshift.io",
	"OAuthClient":               "oauth.openshift.io",
	"Project":                   "project.openshift.io",
	"RoleBindingRestriction":    "authorization.openshift.io",
	"RoleBinding":               "authorization.openshift.io",
	"Role":                      "authorization.openshift.io",
	"Route":                     "route.openshift.io",
	"SecurityContextConstraint": "security.openshift.io",
	"Template":                  "template.openshift.io",
	"UserIdentityMapping":       "user.openshift.io",
	"User":                      "user.openshift.io",
}

func fixOAPIGroupKind(obj map[string]interface{}, gvk *schema.GroupVersionKind) {
	newGroup := fixOAPIGroupKindInTopLevel(obj)

	if len(newGroup) > 0 {
		gvk.Group = newGroup
	}
}

func fixOAPIGroupKindInTopLevel(obj map[string]interface{}) string {
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
