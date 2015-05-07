package bootstrappolicy

// known namespaces
const (
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
	SelfProvisionerRoleName   = "self-provisioner"
	BasicUserRoleName         = "basic-user"
	StatusCheckerRoleName     = "cluster-status"
	DeployerRoleName          = "system:deployer"
	RouterRoleName            = "system:router"
	RegistryRoleName          = "system:registry"
	InternalComponentRoleName = "system:component"
	DeleteTokensRoleName      = "system:delete-tokens"
	WebHooksRoleName          = "system:webhook"

	OpenshiftSharedResourceViewRoleName = "shared-resource-viewer"
)

// RoleBindings
const (
	SelfProvisionerRoleBindingName   = SelfProvisionerRoleName + "s"
	InternalComponentRoleBindingName = InternalComponentRoleName + "s"
	DeployerRoleBindingName          = DeployerRoleName + "s"
	ClusterAdminRoleBindingName      = ClusterAdminRoleName + "s"
	BasicUserRoleBindingName         = BasicUserRoleName + "s"
	DeleteTokensRoleBindingName      = DeleteTokensRoleName + "-binding"
	StatusCheckerRoleBindingName     = StatusCheckerRoleName + "-binding"
	RouterRoleBindingName            = RouterRoleName + "s"
	RegistryRoleBindingName          = RegistryRoleName + "s"
	WebHooksRoleBindingName          = WebHooksRoleName + "s"

	OpenshiftSharedResourceViewRoleBindingName = OpenshiftSharedResourceViewRoleName + "s"
)
