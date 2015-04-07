package bootstrappolicy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func GetBootstrapRoles(masterNamespace, openshiftNamespace string) []authorizationapi.Role {
	masterRoles := GetBootstrapMasterRoles(masterNamespace)
	openshiftRoles := GetBootstrapOpenshiftRoles(openshiftNamespace)
	ret := make([]authorizationapi.Role, 0, len(masterRoles)+len(openshiftRoles))
	ret = append(ret, masterRoles...)
	ret = append(ret, openshiftRoles...)
	return ret
}

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
					Resources: util.NewStringSet("templates", "imagerepositories", "imagerepositorytags"),
				},
			},
		},
	}
}
func GetBootstrapMasterRoles(masterNamespace string) []authorizationapi.Role {
	return []authorizationapi.Role{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      ClusterAdminRoleName,
				Namespace: masterNamespace,
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
				Name:      AdminRoleName,
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "create", "update", "delete"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      EditRoleName,
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "create", "update", "delete"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      ViewRoleName,
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      BasicUserRoleName,
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("users"), ResourceNames: util.NewStringSet("~")},
				{Verbs: util.NewStringSet("list"), Resources: util.NewStringSet("projects")},
				{Verbs: util.NewStringSet("create"), Resources: util.NewStringSet("subjectaccessreviews"), AttributeRestrictions: runtime.EmbeddedObject{&authorizationapi.IsPersonalSubjectAccessReview{}}},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      StatusCheckerRoleName,
				Namespace: masterNamespace,
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
				Name:      DeployerRoleName,
				Namespace: masterNamespace,
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
				Name:      InternalComponentRoleName,
				Namespace: masterNamespace,
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
				Name:      DeleteTokensRoleName,
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("delete"),
					Resources: util.NewStringSet("oauthaccesstoken", "oauthauthorizetoken"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      RouterRoleName,
				Namespace: masterNamespace,
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
				Name:      RegistryRoleName,
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "delete"),
					Resources: util.NewStringSet("images"),
				},
				{
					Verbs:     util.NewStringSet("create"),
					Resources: util.NewStringSet("imagerepositorymappings"),
				},
			},
		},
	}
}

func GetBootstrapRoleBindings(masterNamespace, openshiftNamespace string) []authorizationapi.RoleBinding {
	masterRoleBindings := GetBootstrapMasterRoleBindings(masterNamespace)
	openshiftRoleBindings := GetBootstrapOpenshiftRoleBindings(openshiftNamespace)
	ret := make([]authorizationapi.RoleBinding, 0, len(masterRoleBindings)+len(openshiftRoleBindings))
	ret = append(ret, masterRoleBindings...)
	ret = append(ret, openshiftRoleBindings...)
	return ret
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
func GetBootstrapMasterRoleBindings(masterNamespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      InternalComponentRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      InternalComponentRoleName,
				Namespace: masterNamespace,
			},
			Users:  util.NewStringSet(InternalComponentUsername, InternalComponentKubeUsername),
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      DeployerRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      DeployerRoleName,
				Namespace: masterNamespace,
			},
			Users: util.NewStringSet(DeployerUsername),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      ClusterAdminRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      ClusterAdminRoleName,
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet(ClusterAdminGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      BasicUserRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      BasicUserRoleName,
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet(AuthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      DeleteTokensRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      DeleteTokensRoleName,
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      StatusCheckerRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      StatusCheckerRoleName,
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      RouterRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      RouterRoleName,
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet(RouterGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      RegistryRoleBindingName,
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      RegistryRoleName,
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet(RegistryGroup),
		},
	}
}
