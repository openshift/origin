package openshiftapiserver

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/openshift/origin/pkg/apps/apiserver/registry/deployconfig"
	deploymentconfigetcd "github.com/openshift/origin/pkg/apps/apiserver/registry/deployconfig/etcd"
	buildetcd "github.com/openshift/origin/pkg/build/apiserver/registry/build/etcd"
	buildconfig "github.com/openshift/origin/pkg/build/apiserver/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/apiserver/registry/buildconfig/etcd"
	imagestreametcd "github.com/openshift/origin/pkg/image/apiserver/registry/imagestream/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/apiserver/registry/route"
	routeetcd "github.com/openshift/origin/pkg/route/apiserver/registry/route/etcd"
)

var (
	// originLegacyResources lists all Origin resources that are locked for the legacy v1
	// Origin API. This list should not grow.
	originLegacyResources = sets.NewString(
		"appliedClusterResourceQuotas",
		"buildConfigs",
		"builds",
		"clusterNetworks",
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

	// originLegacySubresources lists all Origin sub-resources that are locked for the
	// legacy v1 Origin API. This list should not grow.
	originLegacySubresources = sets.NewString(
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
			if originLegacyResources.Has(resource) || originLegacySubresources.Has(resource) {
				// We want *some* our legacy resources to orphan by default instead of garbage collecting.
				// Kube only did this for a select few resources which were controller managed and established links
				// via a workload controller.  In openshift, these will all conform to registry.Store so we
				// can actually wrap the "normal" storage here.
				switch storage := s.(type) {
				case *buildetcd.REST:
					legacyStorage[resource] = &buildetcd.LegacyREST{REST: storage}

				case *buildconfigetcd.REST:
					store := *storage.Store
					store.DeleteStrategy = buildconfig.LegacyStrategy
					store.CreateStrategy = buildconfig.LegacyStrategy
					legacyStorage[resource] = &buildconfigetcd.LegacyREST{REST: &buildconfigetcd.REST{Store: &store}}

				case *deploymentconfigetcd.REST:
					store := *storage.Store
					store.CreateStrategy = deployconfig.LegacyStrategy
					store.DeleteStrategy = deployconfig.LegacyStrategy
					legacyStorage[resource] = &deploymentconfigetcd.LegacyREST{REST: &deploymentconfigetcd.REST{Store: &store}}

				case *imagestreametcd.REST:
					legacyStorage[resource] = &imagestreametcd.LegacyREST{REST: storage}
				case *imagestreametcd.LayersREST:
					delete(legacyStorage, resource)

				case *routeetcd.REST:
					store := *storage.Store
					store.Decorator = routeregistry.DecorateLegacyRouteWithEmptyDestinationCACertificates
					legacyStorage[resource] = &routeetcd.LegacyREST{REST: &routeetcd.REST{Store: &store}}

				default:
					legacyStorage[resource] = s
				}
			}
		}
	}
	return legacyStorage
}
