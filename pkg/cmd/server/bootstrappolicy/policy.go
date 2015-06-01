package bootstrappolicy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func GetBootstrapOpenshiftRoles(openshiftNamespace string) []authorizationapi.Role {
	return []authorizationapi.Role{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list"),
					Resources: util.NewStringSet("templates", authorizationapi.ImageGroupName),
				},
			},
		},
	}
}
func GetBootstrapClusterRoles() []authorizationapi.ClusterRole {
	return []authorizationapi.ClusterRole{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet(authorizationapi.VerbAll),
					Resources: util.NewStringSet(authorizationapi.ResourceAll),
				},
				{
					Verbs:           util.NewStringSet(authorizationapi.VerbAll),
					NonResourceURLs: util.NewStringSet(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.ResourceAll),
				},
				{
					Verbs:           util.NewStringSet("get"),
					NonResourceURLs: util.NewStringSet(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: AdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "create", "update", "delete"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName, "projects", "secrets", "pods/proxy"),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "pods/exec", "pods/portforward"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: EditRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "create", "update", "delete"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName, "secrets", "pods/proxy"),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "projects", "pods/exec", "pods/portforward"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ViewRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "projects"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("users"), ResourceNames: util.NewStringSet("~")},
				{Verbs: util.NewStringSet("list"), Resources: util.NewStringSet("projectrequests")},
				{Verbs: util.NewStringSet("list", "get"), Resources: util.NewStringSet("clusterroles")},
				{Verbs: util.NewStringSet("list"), Resources: util.NewStringSet("projects")},
				{Verbs: util.NewStringSet("create"), Resources: util.NewStringSet("subjectaccessreviews"), AttributeRestrictions: runtime.EmbeddedObject{&authorizationapi.IsPersonalSubjectAccessReview{}}},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfProvisionerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: util.NewStringSet("create"), Resources: util.NewStringSet("projectrequests")},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: StatusCheckerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:           util.NewStringSet("get"),
					NonResourceURLs: util.NewStringSet("/healthz", "/version", "/api", "/osapi"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePullerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImageBuilderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "update"),
					Resources: util.NewStringSet("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DeployerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// replicationControllerGetter
					Verbs:     util.NewStringSet("get", "list"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.replicationControllerClient
					// RollingDeploymentStrategy.updaterClient
					Verbs:     util.NewStringSet("get", "update"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.hookExecutor
					// RollingDeploymentStrategy.hookExecutor
					Verbs:     util.NewStringSet("get", "list", "watch", "create"),
					Resources: util.NewStringSet("pods"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: InternalComponentRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet(authorizationapi.VerbAll),
					Resources: util.NewStringSet(authorizationapi.ResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("delete"),
					Resources: util.NewStringSet("oauthaccesstokens", "oauthauthorizetokens"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("routes", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "delete"),
					Resources: util.NewStringSet("images"),
				},
				{
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("imagestreamimages", "imagestreamtags", "imagestreams"),
				},
				{
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("imagestreams"),
				},
				{
					Verbs:     util.NewStringSet("create"),
					Resources: util.NewStringSet("imagestreammappings"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Used to build serviceLister
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("services", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Needed to build serviceLister, to populate env vars for services
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("services"),
				},
				{
					// Nodes can register themselves
					// TODO: restrict to creating a node with the same name they announce
					Verbs:     util.NewStringSet("create", "get", "list", "watch"),
					Resources: util.NewStringSet("nodes"),
				},
				{
					// TODO: restrict to the bound node once supported
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("nodes/status"),
				},

				{
					// TODO: restrict to the bound node as creator once supported
					Verbs:     util.NewStringSet("create", "update"),
					Resources: util.NewStringSet("events"),
				},

				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("pods"),
				},
				{
					// TODO: remove once mirror pods are removed
					// TODO: restrict deletion to mirror pods created by the bound node once supported
					// Needed for the node to create/delete mirror pods
					Verbs:     util.NewStringSet("get", "create", "delete"),
					Resources: util.NewStringSet("pods"),
				},
				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("pods/status"),
				},

				{
					// TODO: restrict to secrets used by pods scheduled on bound node once supported
					// Needed for imagepullsecrets and secret volumes
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("secrets"),
				},

				{
					// TODO: restrict to claims/volumes used by pods scheduled on bound node once supported
					// Needed for persistent volumes
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("persistentvolumeclaims", "persistentvolumes"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("hostsubnets"),
				},
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("nodes"),
				},
				{
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("clusternetworks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNManagerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "create", "delete"),
					Resources: util.NewStringSet("hostsubnets"),
				},
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("nodes"),
				},
				{
					Verbs:     util.NewStringSet("get", "create"),
					Resources: util.NewStringSet("clusternetworks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "create"),
					Resources: util.NewStringSet("buildconfigs/webhooks"),
				},
			},
		},
	}
}

func GetBootstrapOpenshiftRoleBindings(openshiftNamespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleBindingName,
				Namespace: openshiftNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Groups: util.NewStringSet(AuthenticatedGroup),
		},
	}
}

func GetBootstrapClusterRoleBindings() []authorizationapi.ClusterRoleBinding {
	return []authorizationapi.ClusterRoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: InternalComponentRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: InternalComponentRoleName,
			},
			Users: util.NewStringSet(InternalComponentUsername),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DeployerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: DeployerRoleName,
			},
			Users: util.NewStringSet(DeployerUsername),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterAdminRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterAdminRoleName,
			},
			Groups: util.NewStringSet(ClusterAdminGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterReaderRoleName,
			},
			Groups: util.NewStringSet(ClusterReaderGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: BasicUserRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfProvisionerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SelfProvisionerRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: OAuthTokenDeleterRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: StatusCheckerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: StatusCheckerRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RouterRoleName,
			},
			Groups: util.NewStringSet(RouterGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RegistryRoleName,
			},
			Groups: util.NewStringSet(RegistryGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeRoleName,
			},
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeProxierRoleName,
			},
			// Allow node identities to run node proxies
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SDNReaderRoleName,
			},
			// Allow node identities to run SDN plugins
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: WebHooksRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
	}
}
