package bootstrappolicy_test

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

func TestClusterRoles(t *testing.T) {
	currentRoles := bootstrappolicy.GetBootstrapClusterRoles()
	oldRoles := oldGetBootstrapClusterRoles()

	// old roles don't have the SAs appended, so run through them.  The SAs haven't been converted yet
	for i := range oldRoles {
		oldRole := oldRoles[i]
		newRole := currentRoles[i]

		if oldRole.Name != newRole.Name {
			t.Fatalf("%v vs %v", oldRole.Name, newRole.Name)
		}

		// @liggitt don't whine about a temporary test fataling
		if covers, missing := rulevalidation.Covers(oldRole.Rules, newRole.Rules); !covers {
			t.Fatalf("%v/%v: %#v", oldRole.Name, newRole.Name, missing)
		}
		if covers, missing := rulevalidation.Covers(newRole.Rules, oldRole.Rules); !covers {
			t.Fatalf("%v/%v: %#v", oldRole.Name, newRole.Name, missing)
		}
	}
}

func oldGetBootstrapOpenshiftRoles(openshiftNamespace string) []authorizationapi.Role {
	roles := []authorizationapi.Role{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      bootstrappolicy.OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("templates", authorizationapi.ImageGroupName),
				},
				{
					// so anyone can pull from openshift/* image streams
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
	}

	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.NormalizeResources(roles[i].Rules[j].Resources)
		}
	}

	return roles

}

func oldGetBootstrapClusterRoles() []authorizationapi.ClusterRole {
	roles := []authorizationapi.ClusterRole{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ClusterAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{authorizationapi.APIGroupAll},
					Verbs:     sets.NewString(authorizationapi.VerbAll),
					Resources: sets.NewString(authorizationapi.ResourceAll),
				},
				{
					Verbs:           sets.NewString(authorizationapi.VerbAll),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.SudoerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups:     []string{kapi.GroupName},
					Verbs:         sets.NewString("impersonate"),
					Resources:     sets.NewString(authorizationapi.SystemUserResource),
					ResourceNames: sets.NewString(bootstrappolicy.SystemAdminUsername),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ClusterReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.NonEscalatingResourcesGroupName),
				},
				{
					APIGroups: []string{autoscaling.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("horizontalpodautoscalers"),
				},
				{
					APIGroups: []string{batch.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("jobs"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("daemonsets", "jobs", "horizontalpodautoscalers", "replicationcontrollers/scale"),
				},
				{ // permissions to check access.  These creates are non-mutating
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("resourceaccessreviews", "subjectaccessreviews"),
				},
				// Allow read access to node metrics
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString(authorizationapi.NodeMetricsResource),
				},
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString(authorizationapi.NodeStatsResource),
				},
				{
					Verbs:           sets.NewString("get"),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.BuildStrategyDockerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString(authorizationapi.DockerBuildResource),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.BuildStrategyCustomRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString(authorizationapi.CustomBuildResource),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.BuildStrategySourceRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString(authorizationapi.SourceBuildResource),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.BuildStrategyJenkinsPipelineRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString(authorizationapi.JenkinsPipelineBuildResource),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.AdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString(
						authorizationapi.KubeExposedGroupName,
						"secrets",
						"pods/attach", "pods/proxy", "pods/exec", "pods/portforward",
						"services/proxy",
						"replicationcontrollers/scale",
					),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("impersonate"),
					Resources: sets.NewString("serviceaccounts"),
				},
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString(
						authorizationapi.OpenshiftExposedGroupName,
						authorizationapi.PermissionGrantingGroupName,
						"projects",
						"deploymentconfigs/scale",
						"imagestreams/secrets",
					),
				},
				{
					APIGroups: []string{autoscaling.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString("horizontalpodautoscalers"),
				},
				{
					APIGroups: []string{batch.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString("jobs"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("daemonsets"),
				},
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName),
				},
				{
					APIGroups: []string{imageapi.GroupName},
					Verbs:     sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
				// an admin can run routers that write back conditions to the route
				{
					APIGroups: []string{routeapi.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("routes/status"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.EditRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString(
						authorizationapi.KubeExposedGroupName,
						"secrets",
						"pods/attach", "pods/proxy", "pods/exec", "pods/portforward",
						"services/proxy",
						"replicationcontrollers/scale",
					),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("impersonate"),
					Resources: sets.NewString("serviceaccounts"),
				},
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString(
						authorizationapi.OpenshiftExposedGroupName,
						"deploymentconfigs/scale",
						"imagestreams/secrets",
					),
				},
				{
					APIGroups: []string{autoscaling.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString("horizontalpodautoscalers"),
				},
				{
					APIGroups: []string{batch.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString("jobs"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"),
					Resources: sets.NewString("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("daemonsets"),
				},
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "projects"),
				},
				{
					APIGroups: []string{imageapi.GroupName},
					Verbs:     sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ViewRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{api.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "projects"),
				},
				{
					APIGroups: []string{autoscaling.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("horizontalpodautoscalers"),
				},
				{
					APIGroups: []string{batch.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("jobs"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("daemonsets", "jobs", "horizontalpodautoscalers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.BasicUserRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), Resources: sets.NewString("users"), ResourceNames: sets.NewString("~")},
				{Verbs: sets.NewString("list"), Resources: sets.NewString("projectrequests")},
				{Verbs: sets.NewString("list", "get"), Resources: sets.NewString("clusterroles")},
				{Verbs: sets.NewString("list", "watch"), Resources: sets.NewString("projects")},
				{Verbs: sets.NewString("create"), Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
				{Verbs: sets.NewString("create"), Resources: sets.NewString("selfsubjectrulesreviews")},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.SelfProvisionerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("create"), Resources: sets.NewString("projectrequests")},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.StatusCheckerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get"),
					NonResourceURLs: sets.NewString(
						// Health
						"/healthz", "/healthz/*",
					),
				},
				authorizationapi.DiscoveryRule,
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ImageAuditorRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{imageapi.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "patch", "update"),
					Resources: sets.NewString("images"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ImagePullerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			// This role looks like a duplicate of ImageBuilderRole, but the ImageBuilder role is specifically for our builder service accounts
			// if we found another permission needed by them, we'd add it there so the intent is different if you used the ImageBuilderRole
			// you could end up accidentally granting more permissions than you intended.  This is intended to only grant enough powers to
			// push an image to our registry
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ImagePusherRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ImageBuilderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("builds/details"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.ImagePrunerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("delete"),
					Resources: sets.NewString("images"),
				},
				{
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("images", "imagestreams", "pods", "replicationcontrollers", "buildconfigs", "builds", "deploymentconfigs"),
				},
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("imagestreams/status"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.DeployerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// replicationControllerGetter
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.replicationControllerClient
					// RollingDeploymentStrategy.updaterClient
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.hookExecutor
					// RollingDeploymentStrategy.hookExecutor
					Verbs:     sets.NewString("get", "list", "watch", "create"),
					Resources: sets.NewString("pods"),
				},
				{
					// RecreateDeploymentStrategy.hookExecutor
					// RollingDeploymentStrategy.hookExecutor
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("pods/log"),
				},
				{
					// Deployer.After.TagImages
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("imagestreamtags"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.MasterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{authorizationapi.APIGroupAll},
					Verbs:     sets.NewString(authorizationapi.VerbAll),
					Resources: sets.NewString(authorizationapi.ResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.OAuthTokenDeleterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("delete"),
					Resources: sets.NewString("oauthaccesstokens", "oauthauthorizetokens"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.RouterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("routes", "endpoints"),
				},
				// routers write back conditions to the route
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("routes/status"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.RegistryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "delete"),
					Resources: sets.NewString("images"),
				},
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("imagestreamimages", "imagestreamtags", "imagestreams", "imagestreams/secrets"),
				},
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("imagestreams"),
				},
				{
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("imagestreammappings"),
				},
				{
					Verbs:     sets.NewString("list"),
					Resources: sets.NewString("resourcequotas"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.NodeProxierRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Used to build serviceLister
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("services", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.NodeAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// Allow all API calls to the nodes
				{
					Verbs:     sets.NewString("proxy"),
					Resources: sets.NewString("nodes"),
				},
				{
					Verbs:     sets.NewString(authorizationapi.VerbAll),
					Resources: sets.NewString("nodes/proxy", authorizationapi.NodeMetricsResource, authorizationapi.NodeStatsResource, authorizationapi.NodeLogResource),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.NodeReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// Allow read access to node metrics
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString(authorizationapi.NodeMetricsResource),
				},
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString(authorizationapi.NodeStatsResource),
				},
				// TODO: expose other things like /healthz on the node once we figure out non-resource URL policy across systems
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.NodeRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Needed to check API access.  These creates are non-mutating
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"),
				},
				{
					// Needed to build serviceLister, to populate env vars for services
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("services"),
				},
				{
					// Nodes can register themselves
					// TODO: restrict to creating a node with the same name they announce
					Verbs:     sets.NewString("create", "get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				{
					// TODO: restrict to the bound node once supported
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("nodes/status"),
				},

				{
					// TODO: restrict to the bound node as creator once supported
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},

				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("pods"),
				},
				{
					// TODO: remove once mirror pods are removed
					// TODO: restrict deletion to mirror pods created by the bound node once supported
					// Needed for the node to create/delete mirror pods
					Verbs:     sets.NewString("get", "create", "delete"),
					Resources: sets.NewString("pods"),
				},
				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("pods/status"),
				},

				{
					// TODO: restrict to secrets and configmaps used by pods scheduled on bound node once supported
					// Needed for imagepullsecrets, rbd/ceph and secret volumes, and secrets in envs
					// Needed for configmap volume and envs
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("secrets", "configmaps"),
				},
				{
					// TODO: restrict to claims/volumes used by pods scheduled on bound node once supported
					// Needed for persistent volumes
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("persistentvolumeclaims", "persistentvolumes"),
				},
				{
					// TODO: restrict to namespaces of pods scheduled on bound node once supported
					// TODO: change glusterfs to use DNS lookup so this isn't needed?
					// Needed for glusterfs volumes
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("endpoints"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.SDNReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("hostsubnets"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("netnamespaces"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("clusternetworks"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("namespaces"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.SDNManagerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch", "create", "delete"),
					Resources: sets.NewString("hostsubnets"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch", "create", "delete"),
					Resources: sets.NewString("netnamespaces"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString("clusternetworks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.WebHooksRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString("buildconfigs/webhooks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.DiscoveryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.DiscoveryRule,
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.RegistryAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"),
					APIGroups: []string{imageapi.GroupName},
					Resources: sets.NewString("imagestreamimages", "imagestreamimports", "imagestreammappings", "imagestreams", "imagestreams/secrets", "imagestreamtags"),
				},
				{
					Verbs:     sets.NewString("get", "update"),
					APIGroups: []string{imageapi.GroupName},
					Resources: sets.NewString("imagestreams/layers"),
				},
				{
					Verbs:     sets.NewString("create"),
					APIGroups: []string{authorizationapi.GroupName},
					Resources: sets.NewString("localresourceaccessreviews", "localsubjectaccessreviews", "resourceaccessreviews", "subjectaccessreviews"),
				},
				{
					Verbs:     sets.NewString("create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"),
					APIGroups: []string{authorizationapi.GroupName},
					Resources: sets.NewString("rolebindings", "roles"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					APIGroups: []string{authorizationapi.GroupName},
					Resources: sets.NewString("policies", "policybindings"),
				},
				{
					Verbs:     sets.NewString("get"),
					APIGroups: []string{kapi.GroupName},
					Resources: sets.NewString("namespaces"),
				},
				{
					Verbs:     sets.NewString("get", "delete"),
					APIGroups: []string{projectapi.GroupName},
					Resources: sets.NewString("projects"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.RegistryEditorRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"),
					APIGroups: []string{imageapi.GroupName},
					Resources: sets.NewString("imagestreamimages", "imagestreamimports", "imagestreammappings", "imagestreams", "imagestreams/secrets", "imagestreamtags"),
				},
				{
					Verbs:     sets.NewString("get", "update"),
					APIGroups: []string{imageapi.GroupName},
					Resources: sets.NewString("imagestreams/layers"),
				},
				{
					Verbs:     sets.NewString("get"),
					APIGroups: []string{kapi.GroupName},
					Resources: sets.NewString("namespaces"),
				},
				{
					Verbs:     sets.NewString("get"),
					APIGroups: []string{projectapi.GroupName},
					Resources: sets.NewString("projects"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: bootstrappolicy.RegistryViewerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					APIGroups: []string{imageapi.GroupName},
					Resources: sets.NewString("imagestreamimages", "imagestreamimports", "imagestreammappings", "imagestreams", "imagestreamtags"),
				},
				{
					Verbs:     sets.NewString("get"),
					APIGroups: []string{imageapi.GroupName},
					Resources: sets.NewString("imagestreams/layers"),
				},
				{
					Verbs:     sets.NewString("get"),
					APIGroups: []string{kapi.GroupName},
					Resources: sets.NewString("namespaces"),
				},
				{
					Verbs:     sets.NewString("get"),
					APIGroups: []string{projectapi.GroupName},
					Resources: sets.NewString("projects"),
				},
			},
		},
	}

	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.NormalizeResources(roles[i].Rules[j].Resources)
		}
	}

	versionedRoles := []authorizationapiv1.ClusterRole{}
	for i := range roles {
		newRole := &authorizationapiv1.ClusterRole{}
		kapi.Scheme.Convert(&roles[i], newRole)
		versionedRoles = append(versionedRoles, *newRole)
	}

	roundtrippedRoles := []authorizationapi.ClusterRole{}
	for i := range versionedRoles {
		newRole := &authorizationapi.ClusterRole{}
		kapi.Scheme.Convert(&versionedRoles[i], newRole)
		roundtrippedRoles = append(roundtrippedRoles, *newRole)
	}

	return roundtrippedRoles
}
