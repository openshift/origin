package testrestmapper

import "k8s.io/apimachinery/pkg/runtime/schema"

func init() {
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "SubjectAccessReview"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ResourceAccessReview"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ClusterRole"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ClusterRoleBinding"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "Image"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ImageSignature"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "OAuthAccessToken"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "OAuthAuthorizeToken"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "OAuthClient"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "OAuthClientAuthorization"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "OAuthRedirectReference"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "Project"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ProjectRequest"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ClusterNetwork"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "HostSubnet"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "NetNamespace"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "BrokerTemplateInstance"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "User"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "Identity"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "UserIdentityMapping"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "Group"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "SecurityContextConstraints"}] = true
	rootScopedKinds[schema.GroupKind{Group: "", Kind: "ClusterResourceQuota"}] = true

	rootScopedKinds[schema.GroupKind{Group: "authorization.openshift.io", Kind: "SubjectAccessReview"}] = true
	rootScopedKinds[schema.GroupKind{Group: "authorization.openshift.io", Kind: "ResourceAccessReview"}] = true
	rootScopedKinds[schema.GroupKind{Group: "authorization.openshift.io", Kind: "ClusterRole"}] = true
	rootScopedKinds[schema.GroupKind{Group: "authorization.openshift.io", Kind: "ClusterRoleBinding"}] = true
	rootScopedKinds[schema.GroupKind{Group: "image.openshift.io", Kind: "Image"}] = true
	rootScopedKinds[schema.GroupKind{Group: "image.openshift.io", Kind: "ImageSignature"}] = true
	rootScopedKinds[schema.GroupKind{Group: "oauth.openshift.io", Kind: "OAuthAccessToken"}] = true
	rootScopedKinds[schema.GroupKind{Group: "oauth.openshift.io", Kind: "OAuthAuthorizeToken"}] = true
	rootScopedKinds[schema.GroupKind{Group: "oauth.openshift.io", Kind: "OAuthClient"}] = true
	rootScopedKinds[schema.GroupKind{Group: "oauth.openshift.io", Kind: "OAuthClientAuthorization"}] = true
	rootScopedKinds[schema.GroupKind{Group: "oauth.openshift.io", Kind: "OAuthRedirectReference"}] = true
	rootScopedKinds[schema.GroupKind{Group: "project.openshift.io", Kind: "Project"}] = true
	rootScopedKinds[schema.GroupKind{Group: "project.openshift.io", Kind: "ProjectRequest"}] = true
	rootScopedKinds[schema.GroupKind{Group: "network.openshift.io", Kind: "ClusterNetwork"}] = true
	rootScopedKinds[schema.GroupKind{Group: "network.openshift.io", Kind: "HostSubnet"}] = true
	rootScopedKinds[schema.GroupKind{Group: "network.openshift.io", Kind: "NetNamespace"}] = true
	rootScopedKinds[schema.GroupKind{Group: "template.openshift.io", Kind: "BrokerTemplateInstance"}] = true
	rootScopedKinds[schema.GroupKind{Group: "user.openshift.io", Kind: "User"}] = true
	rootScopedKinds[schema.GroupKind{Group: "user.openshift.io", Kind: "Identity"}] = true
	rootScopedKinds[schema.GroupKind{Group: "user.openshift.io", Kind: "UserIdentityMapping"}] = true
	rootScopedKinds[schema.GroupKind{Group: "user.openshift.io", Kind: "Group"}] = true
	rootScopedKinds[schema.GroupKind{Group: "security.openshift.io", Kind: "SecurityContextConstraints"}] = true
	rootScopedKinds[schema.GroupKind{Group: "quota.openshift.io", Kind: "ClusterResourceQuota"}] = true
}
