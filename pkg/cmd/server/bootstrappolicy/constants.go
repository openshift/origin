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

	MasterUnqualifiedUsername   = "openshift-master"
	RouterUnqualifiedUsername   = "openshift-router"
	RegistryUnqualifiedUsername = "openshift-registry"

	MasterUsername   = "system:" + MasterUnqualifiedUsername
	RouterUsername   = "system:" + RouterUnqualifiedUsername
	RegistryUsername = "system:" + RegistryUnqualifiedUsername
)

// groups
const (
	UnauthenticatedUsername = "system:anonymous"

	AuthenticatedGroup   = "system:authenticated"
	UnauthenticatedGroup = "system:unauthenticated"
	ClusterAdminGroup    = "system:cluster-admins"
	ClusterReaderGroup   = "system:cluster-readers"
	MastersGroup         = "system:masters"
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
	MasterRoleName            = "system:master"
	NodeRoleName              = "system:node"
	NodeProxierRoleName       = "system:node-proxier"
	SDNReaderRoleName         = "system:sdn-reader"
	SDNManagerRoleName        = "system:sdn-manager"
	OAuthTokenDeleterRoleName = "system:oauth-token-deleter"
	WebHooksRoleName          = "system:webhook"

	OpenshiftSharedResourceViewRoleName = "shared-resource-viewer"
)

// RoleBindings
const (
	SelfProvisionerRoleBindingName   = SelfProvisionerRoleName + "s"
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
	MasterRoleBindingName            = MasterRoleName + "s"
	NodeRoleBindingName              = NodeRoleName + "s"
	NodeProxierRoleBindingName       = NodeProxierRoleName + "s"
	SDNReaderRoleBindingName         = SDNReaderRoleName + "s"
	SDNManagerRoleBindingName        = SDNManagerRoleName + "s"
	WebHooksRoleBindingName          = WebHooksRoleName + "s"

	OpenshiftSharedResourceViewRoleBindingName = OpenshiftSharedResourceViewRoleName + "s"
)
