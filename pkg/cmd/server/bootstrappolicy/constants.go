package bootstrappolicy

// known namespaces
const (
	DefaultOpenShiftSharedResourcesNamespace = "openshift"
	DefaultOpenShiftInfraNamespace           = "openshift-infra"
)

// users
const (
	DefaultServiceAccountName  = "default"
	BuilderServiceAccountName  = "builder"
	DeployerServiceAccountName = "deployer"

	InfraBuildControllerServiceAccountName       = "build-controller"
	InfraReplicationControllerServiceAccountName = "replication-controller"
	InfraDeploymentControllerServiceAccountName  = "deployment-controller"

	RouterUnqualifiedUsername   = "openshift-router"
	RegistryUnqualifiedUsername = "openshift-registry"

	RouterUsername   = "system:" + RouterUnqualifiedUsername
	RegistryUsername = "system:" + RegistryUnqualifiedUsername
)

// groups
const (
	UnauthenticatedUsername   = "system:anonymous"
	InternalComponentUsername = "system:openshift-client"

	AuthenticatedGroup   = "system:authenticated"
	UnauthenticatedGroup = "system:unauthenticated"
	ClusterAdminGroup    = "system:cluster-admins"
	ClusterReaderGroup   = "system:cluster-readers"
	NodesGroup           = "system:nodes"
	RouterGroup          = "system:routers"
	RegistryGroup        = "system:registries"
)

// Roles
const (
	ClusterAdminRoleName    = "cluster-admin"
	ClusterReaderRoleName   = "cluster-reader"
	AdminRoleName           = "admin"
	EditRoleName            = "edit"
	ViewRoleName            = "view"
	SelfProvisionerRoleName = "self-provisioner"
	BasicUserRoleName       = "basic-user"
	StatusCheckerRoleName   = "cluster-status"

	BuildControllerRoleName       = "system:build-controller"
	ReplicationControllerRoleName = "system:replication-controller"
	DeploymentControllerRoleName  = "system:deployment-controller"

	ImagePullerRoleName       = "system:image-puller"
	ImageBuilderRoleName      = "system:image-builder"
	ImagePrunerRoleName       = "system:image-pruner"
	DeployerRoleName          = "system:deployer"
	RouterRoleName            = "system:router"
	RegistryRoleName          = "system:registry"
	NodeRoleName              = "system:node"
	NodeProxierRoleName       = "system:node-proxier"
	SDNReaderRoleName         = "system:sdn-reader"
	SDNManagerRoleName        = "system:sdn-manager"
	InternalComponentRoleName = "system:component"
	OAuthTokenDeleterRoleName = "system:oauth-token-deleter"
	WebHooksRoleName          = "system:webhook"

	OpenshiftSharedResourceViewRoleName = "shared-resource-viewer"
)

// RoleBindings
const (
	SelfProvisionerRoleBindingName   = SelfProvisionerRoleName + "s"
	InternalComponentRoleBindingName = InternalComponentRoleName + "s"
	DeployerRoleBindingName          = DeployerRoleName + "s"
	ClusterAdminRoleBindingName      = ClusterAdminRoleName + "s"
	ClusterReaderRoleBindingName     = ClusterReaderRoleName + "s"
	BasicUserRoleBindingName         = BasicUserRoleName + "s"
	OAuthTokenDeleterRoleBindingName = OAuthTokenDeleterRoleName + "s"
	StatusCheckerRoleBindingName     = StatusCheckerRoleName + "-binding"
	ImagePullerRoleBindingName       = ImagePullerRoleName + "s"
	ImageBuilderRoleBindingName      = ImageBuilderRoleName + "s"
	RouterRoleBindingName            = RouterRoleName + "s"
	RegistryRoleBindingName          = RegistryRoleName + "s"
	NodeRoleBindingName              = NodeRoleName + "s"
	NodeProxierRoleBindingName       = NodeProxierRoleName + "s"
	SDNReaderRoleBindingName         = SDNReaderRoleName + "s"
	SDNManagerRoleBindingName        = SDNManagerRoleName + "s"
	WebHooksRoleBindingName          = WebHooksRoleName + "s"

	OpenshiftSharedResourceViewRoleBindingName = OpenshiftSharedResourceViewRoleName + "s"
)
