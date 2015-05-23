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
					Resources: util.NewStringSet("imagestreams"),
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
					Resources: util.NewStringSet("imagestreams"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DeployerRoleName,
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
				Name: DeleteTokensRoleName,
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
					// TODO: remove "create" once we re-enable user authentication in the registry
					Verbs:     util.NewStringSet("create", "update"),
					Resources: util.NewStringSet("imagestreams"),
				},
				{
					Verbs:     util.NewStringSet("create"),
					Resources: util.NewStringSet("imagerepositorymappings", "imagestreammappings"),
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
			Users:  util.NewStringSet(InternalComponentUsername, InternalComponentKubeUsername),
			Groups: util.NewStringSet(NodesGroup),
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
				Name: DeleteTokensRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: DeleteTokensRoleName,
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
				Name: WebHooksRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: WebHooksRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
	}
}
