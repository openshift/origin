package authorizer

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func GetBootstrapPolicy(masterNamespace string) *authorizationapi.Policy {
	return &authorizationapi.Policy{
		ObjectMeta: kapi.ObjectMeta{
			Name:              authorizationapi.PolicyName,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
		},
		LastModified: util.Now(),
		Roles: map[string]authorizationapi.Role{
			"cluster-admin": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
					},
				},
			},
			"admin": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "admin",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
						Resources: []string{authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName},
					},
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName},
					},
				},
			},
			"edit": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "edit",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
						Resources: []string{authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName},
					},
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{authorizationapi.KubeAllGroupName},
					},
				},
			},
			"view": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "view",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName},
					},
				},
			},
			"basic-user": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "view-self",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{Verbs: []string{"get"}, Resources: []string{"users"}, ResourceNames: util.NewStringSet("~")},
					{Verbs: []string{"list"}, Resources: []string{"projects"}},
				},
			},
			"system:deployer": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:deployer",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
					},
				},
			},
			"system:component": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:component",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
					},
				},
			},
			"system:delete-tokens": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:delete-tokens",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"delete"},
						Resources: []string{"oauthaccesstoken", "oauthauthorizetoken"},
					},
				},
			},
		},
	}
}

func GetBootstrapPolicyBinding(masterNamespace string) *authorizationapi.PolicyBinding {
	return &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:              masterNamespace,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
		},
		LastModified: util.Now(),
		PolicyRef:    kapi.ObjectReference{Namespace: masterNamespace},
		RoleBindings: map[string]authorizationapi.RoleBinding{
			"system:component-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:component-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "system:component",
					Namespace: masterNamespace,
				},
				UserNames: []string{"system:openshift-client", "system:kube-client"},
			},
			"system:deployer-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:deployer",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "system:deployer",
					Namespace: masterNamespace,
				},
				UserNames: []string{"system:openshift-deployer"},
			},
			"cluster-admin-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-admin-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				UserNames: []string{"system:admin"},
			},
			"basic-user-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "basic-user-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "basic-user",
					Namespace: masterNamespace,
				},
				GroupNames: []string{"system:authenticated"},
			},
			"insecure-cluster-admin-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "insecure-cluster-admin-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				// TODO until we decide to enforce policy, simply allow every one access
				GroupNames: []string{"system:authenticated", "system:unauthenticated"},
			},
			"system:delete-tokens-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:delete-tokens-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "system:delete-tokens",
					Namespace: masterNamespace,
				},
				GroupNames: []string{"system:authenticated", "system:unauthenticated"},
			},
		},
	}
}
