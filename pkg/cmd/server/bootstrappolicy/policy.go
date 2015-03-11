package bootstrappolicy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
				Name:      "shared-resource-viewer",
				Namespace: openshiftNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list"),
					Resources: util.NewStringSet("templates", "imageRepositories", "imageRepositoryTags"),
				},
			},
		},
	}
}
func GetBootstrapMasterRoles(masterNamespace string) []authorizationapi.Role {
	return []authorizationapi.Role{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "cluster-admin",
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
				Name:      "admin",
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "redirect", "create", "update", "delete"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "redirect"),
					Resources: util.NewStringSet(authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "edit",
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "redirect", "create", "update", "delete"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "redirect"),
					Resources: util.NewStringSet(authorizationapi.KubeAllGroupName),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "view",
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "redirect"),
					Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "basic-user",
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("users"), ResourceNames: util.NewStringSet("~")},
				{Verbs: util.NewStringSet("list"), Resources: util.NewStringSet("projects")},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "cluster-status",
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
				Name:      "system:deployer",
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
				Name:      "system:component",
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
				Name:      "system:delete-tokens",
				Namespace: masterNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("delete"),
					Resources: util.NewStringSet("oauthaccesstoken", "oauthauthorizetoken"),
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
				Name:      "shared-resource-viewer-binding",
				Namespace: openshiftNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "shared-resource-viewer",
				Namespace: openshiftNamespace,
			},
			Groups: util.NewStringSet("system:authenticated"),
		},
	}
}
func GetBootstrapMasterRoleBindings(masterNamespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "system:component-binding",
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "system:component",
				Namespace: masterNamespace,
			},
			Users: util.NewStringSet("system:openshift-client", "system:kube-client"),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "system:deployer-binding",
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "system:deployer",
				Namespace: masterNamespace,
			},
			Users: util.NewStringSet("system:openshift-deployer"),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "cluster-admin-binding",
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "cluster-admin",
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet("system:cluster-admins"),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "basic-user-binding",
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "basic-user",
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet("system:authenticated"),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "system:delete-tokens-binding",
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "system:delete-tokens",
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet("system:authenticated", "system:unauthenticated"),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "cluster-status-binding",
				Namespace: masterNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      "cluster-status",
				Namespace: masterNamespace,
			},
			Groups: util.NewStringSet("system:authenticated", "system:unauthenticated"),
		},
	}
}
