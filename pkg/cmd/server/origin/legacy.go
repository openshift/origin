package origin

import (
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"
)

var (
	// OriginLegacyKinds lists all kinds that are locked to the legacy Origin API schema.
	// This list should not grow and adding a new types to the locked Origin API schema will
	// cause a unit test failure.
	OriginLegacyKinds = sets.NewString(
		"AppliedClusterResourceQuota",
		"AppliedClusterResourceQuotaList",
		"BinaryBuildRequestOptions",
		"Build",
		"BuildConfig",
		"BuildConfigList",
		"BuildList",
		"BuildLog",
		"BuildLogOptions",
		"BuildRequest",
		"ClusterNetwork",
		"ClusterNetworkList",
		"ClusterPolicy",
		"ClusterPolicyBinding",
		"ClusterPolicyBindingList",
		"ClusterPolicyList",
		"ClusterResourceQuota",
		"ClusterResourceQuotaList",
		"ClusterRole",
		"ClusterRoleBinding",
		"ClusterRoleBindingList",
		"ClusterRoleList",
		"DeploymentConfig",
		"DeploymentConfigList",
		"DeploymentConfigRollback",
		"DeploymentLog",
		"DeploymentLogOptions",
		"DeploymentRequest",
		"EgressNetworkPolicy",
		"EgressNetworkPolicyList",
		"Group",
		"GroupList",
		"HostSubnet",
		"HostSubnetList",
		"Identity",
		"IdentityList",
		"Image",
		"ImageList",
		"ImageSignature",
		"ImageStream",
		"ImageStreamImage",
		"ImageStreamImport",
		"ImageStreamList",
		"ImageStreamMapping",
		"ImageStreamTag",
		"ImageStreamTagList",
		"IsPersonalSubjectAccessReview",
		"LocalResourceAccessReview",
		"LocalSubjectAccessReview",
		"NetNamespace",
		"NetNamespaceList",
		"OAuthAccessToken",
		"OAuthAccessTokenList",
		"OAuthAuthorizeToken",
		"OAuthAuthorizeTokenList",
		"OAuthClient",
		"OAuthClientAuthorization",
		"OAuthClientAuthorizationList",
		"OAuthClientList",
		"OAuthRedirectReference",
		"PodSecurityPolicyReview",
		"PodSecurityPolicySelfSubjectReview",
		"PodSecurityPolicySubjectReview",
		"Policy",
		"PolicyBinding",
		"PolicyBindingList",
		"PolicyList",
		"ProcessedTemplate",
		"Project",
		"ProjectList",
		"ProjectRequest",
		"ResourceAccessReview",
		"ResourceAccessReviewResponse",
		"Role",
		"RoleBinding",
		"RoleBindingList",
		"RoleBindingRestriction",
		"RoleBindingRestrictionList",
		"RoleList",
		"Route",
		"RouteList",
		"SelfSubjectRulesReview",
		"SubjectAccessReview",
		"SubjectAccessReviewResponse",
		"SubjectRulesReview",
		"Template",
		"TemplateConfig",
		"TemplateList",
		"User",
		"UserIdentityMapping",
		"UserList",
	)

	// OriginLegacyResources lists all Origin resources that are locked for the legacy v1
	// Origin API. This list should not grow.
	OriginLegacyResources = sets.NewString(
		"appliedClusterResourceQuotas",
		"buildConfigs",
		"builds",
		"clusterNetworks",
		"clusterPolicies",
		"clusterPolicyBindings",
		"clusterResourceQuotas",
		"clusterRoleBindings",
		"clusterRoles",
		"deploymentConfigRollbacks",
		"deploymentConfigs",
		"egressNetworkPolicies",
		"groups",
		"hostSubnets",
		"identities",
		"imageStreamImages",
		"imageStreamImports",
		"imageStreamMappings",
		"imageStreamTags",
		"imageStreams",
		"images",
		"imagesignatures",
		"localResourceAccessReviews",
		"localSubjectAccessReviews",
		"netNamespaces",
		"oAuthAccessTokens",
		"oAuthAuthorizeTokens",
		"oAuthClientAuthorizations",
		"oAuthClients",
		"podSecurityPolicyReviews",
		"podSecurityPolicySelfSubjectReviews",
		"podSecurityPolicySubjectReviews",
		"policies",
		"policyBindings",
		"processedTemplates",
		"projectRequests",
		"projects",
		"resourceAccessReviews",
		"roleBindingRestrictions",
		"roleBindings",
		"roles",
		"routes",
		"selfSubjectRulesReviews",
		"subjectAccessReviews",
		"subjectRulesReviews",
		"templates",
		"userIdentityMappings",
		"users",
	)

	// OriginLegacySubresources lists all Origin sub-resources that are locked for the
	// legacy v1 Origin API. This list should not grow.
	OriginLegacySubresources = sets.NewString(
		"clusterResourceQuotas/status",
		"processedTemplates",
		"imageStreams/status",
		"imageStreams/secrets",
		"generateDeploymentConfigs",
		"deploymentConfigs/log",
		"deploymentConfigs/instantiate",
		"deploymentConfigs/scale",
		"deploymentConfigs/status",
		"deploymentConfigs/rollback",
		"routes/status",
		"builds/clone",
		"builds/log",
		"builds/details",
		"buildConfigs/webhooks",
		"buildConfigs/instantiate",
		"buildConfigs/instantiatebinary",
	)
)

// LegacyStorage returns a storage for locked legacy types.
func LegacyStorage(storage map[unversioned.GroupVersion]map[string]rest.Storage) map[string]rest.Storage {
	legacyStorage := map[string]rest.Storage{}
	for _, gvStorage := range storage {
		for resource, s := range gvStorage {
			if OriginLegacyResources.Has(resource) || OriginLegacySubresources.Has(resource) {
				legacyStorage[resource] = s
			}
		}
	}
	return legacyStorage
}
