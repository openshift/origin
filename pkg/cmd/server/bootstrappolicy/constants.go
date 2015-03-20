package bootstrappolicy

import ()

// known namespaces
const (
	DefaultMasterAuthorizationNamespace      = "master"
	DefaultOpenShiftSharedResourcesNamespace = "openshift"
)

// users
const (
	RouterUnqualifiedUsername   = "openshift-router"
	RegistryUnqualifiedUsername = "openshift-registry"

	RouterUsername   = "system:" + RouterUnqualifiedUsername
	RegistryUsername = "system:" + RegistryUnqualifiedUsername
)

// groups
const (
	UnauthenticatedUsername       = "system:anonymous"
	InternalComponentUsername     = "system:openshift-client"
	InternalComponentKubeUsername = "system:kube-client"
	DeployerUsername              = "system:openshift-deployer"

	AuthenticatedGroup   = "system:authenticated"
	UnauthenticatedGroup = "system:unauthenticated"
	ClusterAdminGroup    = "system:cluster-admins"
	NodesGroup           = "system:nodes"
	RouterGroup          = "system:routers"
	RegistryGroup        = "system:registries"
)

// Roles
const (
	ClusterAdminRoleName      = "cluster-admin"
	AdminRoleName             = "admin"
	EditRoleName              = "edit"
	ViewRoleName              = "view"
	BasicUserRoleName         = "basic-user"
	StatusCheckerRoleName     = "cluster-status"
	DeployerRoleName          = "system:deployer"
	RouterRoleName            = "system:router"
	RegistryRoleName          = "system:registry"
	InternalComponentRoleName = "system:component"
	DeleteTokensRoleName      = "system:delete-tokens"

	OpenshiftSharedResourceViewRoleName = "shared-resource-viewer"
)

// RoleBindings
const (
	InternalComponentRoleBindingName = InternalComponentRoleName + "-binding"
	DeployerRoleBindingName          = DeployerRoleName + "-binding"
	ClusterAdminRoleBindingName      = ClusterAdminRoleName + "-binding"
	BasicUserRoleBindingName         = BasicUserRoleName + "-binding"
	DeleteTokensRoleBindingName      = DeleteTokensRoleName + "-binding"
	StatusCheckerRoleBindingName     = StatusCheckerRoleName + "-binding"
	RouterRoleBindingName            = RouterRoleName + "-binding"
	RegistryRoleBindingName          = RegistryRoleName + "-binding"

	OpenshiftSharedResourceViewRoleBindingName = OpenshiftSharedResourceViewRoleName + "-binding"
)
