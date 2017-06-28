package origin

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/registry/rest"

	buildconfig "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	deploymentconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	routeetcd "github.com/openshift/origin/pkg/route/registry/route/etcd"
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
func LegacyStorage(storage map[schema.GroupVersion]map[string]rest.Storage) map[string]rest.Storage {
	legacyStorage := map[string]rest.Storage{}
	for _, gvStorage := range storage {
		for resource, s := range gvStorage {
			if OriginLegacyResources.Has(resource) || OriginLegacySubresources.Has(resource) {
				// We want *some* our legacy resources to orphan by default instead of garbage collecting.
				// Kube only did this for a select few resources which were controller managed and established links
				// via a workload controller.  In openshift, these will all conform to registry.Store so we
				// can actually wrap the "normal" storage here.
				switch resource {
				case "buildConfigs":
					restStorage := s.(*buildconfigetcd.REST)
					store := *restStorage.Store
					store.DeleteStrategy = buildconfig.LegacyStrategy
					store.CreateStrategy = buildconfig.LegacyStrategy
					legacyStorage[resource] = &buildconfigetcd.REST{Store: &store}
				case "deploymentConfigs":
					restStorage := s.(*deploymentconfigetcd.REST)
					store := *restStorage.Store
					restStorage.DeleteStrategy = orphanByDefault(store.DeleteStrategy)
					legacyStorage[resource] = &deploymentconfigetcd.REST{Store: &store}
				case "routes":
					restStorage := s.(*routeetcd.REST)
					store := *restStorage.Store
					store.Decorator = routeregistry.DecorateLegacyRouteWithEmptyDestinationCACertificates
					legacyStorage[resource] = &routeetcd.REST{Store: &store}

				default:
					legacyStorage[resource] = s
				}

			}
		}
	}
	return legacyStorage
}

type orphaningDeleter struct {
	rest.RESTDeleteStrategy
}

func (o orphaningDeleter) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.OrphanDependents
}

func orphanByDefault(in rest.RESTDeleteStrategy) rest.RESTDeleteStrategy {
	return orphaningDeleter{in}
}
