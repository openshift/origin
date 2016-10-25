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

	MasterUnqualifiedUsername   = "openshift-master"
	RouterUnqualifiedUsername   = "openshift-router"
	RegistryUnqualifiedUsername = "openshift-registry"

	MasterUsername      = "system:" + MasterUnqualifiedUsername
	RouterUsername      = "system:" + RouterUnqualifiedUsername
	RegistryUsername    = "system:" + RegistryUnqualifiedUsername
	SystemAdminUsername = "system:admin"

	// Not granted any API permissions, just an identity for a client certificate for the API proxy to use
	// Should not be changed without considering impact to pods that may be verifying this identity by default
	MasterProxyUnqualifiedUsername = "master-proxy"
	MasterProxyUsername            = "system:" + MasterProxyUnqualifiedUsername

	// Previous versions used this as the username for the master to connect to the kubelet
	// This should remain in the default role bindings for the NodeAdmin role
	LegacyMasterKubeletAdminClientUsername = "system:master"
	MasterKubeletAdminClientUsername       = "system:openshift-node-admin"
)

// groups
const (
	UnauthenticatedUsername = "system:anonymous"

	AuthenticatedGroup      = "system:authenticated"
	AuthenticatedOAuthGroup = "system:authenticated:oauth"
	UnauthenticatedGroup    = "system:unauthenticated"
	ClusterAdminGroup       = "system:cluster-admins"
	ClusterReaderGroup      = "system:cluster-readers"
	MastersGroup            = "system:masters"
	NodesGroup              = "system:nodes"
	NodeAdminsGroup         = "system:node-admins"
	NodeReadersGroup        = "system:node-readers"
	RouterGroup             = "system:routers"
	RegistryGroup           = "system:registries"
)

// Roles
const (
	ClusterAdminRoleName       = "cluster-admin"
	SudoerRoleName             = "sudoer"
	ClusterReaderRoleName      = "cluster-reader"
	StorageAdminRoleName       = "storage-admin"
	AdminRoleName              = "admin"
	EditRoleName               = "edit"
	ViewRoleName               = "view"
	SelfProvisionerRoleName    = "self-provisioner"
	BasicUserRoleName          = "basic-user"
	StatusCheckerRoleName      = "cluster-status"
	SelfAccessReviewerRoleName = "self-access-reviewer"

	RegistryAdminRoleName  = "registry-admin"
	RegistryViewerRoleName = "registry-viewer"
	RegistryEditorRoleName = "registry-editor"

	BuildStrategyDockerRoleName          = "system:build-strategy-docker"
	BuildStrategyCustomRoleName          = "system:build-strategy-custom"
	BuildStrategySourceRoleName          = "system:build-strategy-source"
	BuildStrategyJenkinsPipelineRoleName = "system:build-strategy-jenkinspipeline"

	ImageAuditorRoleName      = "system:image-auditor"
	ImagePullerRoleName       = "system:image-puller"
	ImagePusherRoleName       = "system:image-pusher"
	ImageBuilderRoleName      = "system:image-builder"
	ImagePrunerRoleName       = "system:image-pruner"
	ImageSignerRoleName       = "system:image-signer"
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
	DiscoveryRoleName         = "system:discovery"

	// NodeAdmin has full access to the API provided by the kubelet
	NodeAdminRoleName = "system:node-admin"
	// NodeReader has read access to the metrics and stats provided by the kubelet
	NodeReaderRoleName = "system:node-reader"

	OpenshiftSharedResourceViewRoleName = "shared-resource-viewer"
)

// RoleBindings
const (
	SelfAccessReviewerRoleBindingName = SelfAccessReviewerRoleName + "s"
	SelfProvisionerRoleBindingName    = SelfProvisionerRoleName + "s"
	DeployerRoleBindingName           = DeployerRoleName + "s"
	ClusterAdminRoleBindingName       = ClusterAdminRoleName + "s"
	ClusterReaderRoleBindingName      = ClusterReaderRoleName + "s"
	BasicUserRoleBindingName          = BasicUserRoleName + "s"
	OAuthTokenDeleterRoleBindingName  = OAuthTokenDeleterRoleName + "s"
	StatusCheckerRoleBindingName      = StatusCheckerRoleName + "-binding"
	ImagePullerRoleBindingName        = ImagePullerRoleName + "s"
	ImageBuilderRoleBindingName       = ImageBuilderRoleName + "s"
	RouterRoleBindingName             = RouterRoleName + "s"
	RegistryRoleBindingName           = RegistryRoleName + "s"
	MasterRoleBindingName             = MasterRoleName + "s"
	NodeRoleBindingName               = NodeRoleName + "s"
	NodeProxierRoleBindingName        = NodeProxierRoleName + "s"
	NodeAdminRoleBindingName          = NodeAdminRoleName + "s"
	NodeReaderRoleBindingName         = NodeReaderRoleName + "s"
	SDNReaderRoleBindingName          = SDNReaderRoleName + "s"
	SDNManagerRoleBindingName         = SDNManagerRoleName + "s"
	WebHooksRoleBindingName           = WebHooksRoleName + "s"
	DiscoveryRoleBindingName          = DiscoveryRoleName + "-binding"
	RegistryAdminRoleBindingName      = RegistryAdminRoleName + "s"
	RegistryViewerRoleBindingName     = RegistryViewerRoleName + "s"
	RegistryEditorRoleBindingName     = RegistryEditorRoleName + "s"

	BuildStrategyDockerRoleBindingName          = BuildStrategyDockerRoleName + "-binding"
	BuildStrategyCustomRoleBindingName          = BuildStrategyCustomRoleName + "-binding"
	BuildStrategySourceRoleBindingName          = BuildStrategySourceRoleName + "-binding"
	BuildStrategyJenkinsPipelineRoleBindingName = BuildStrategyJenkinsPipelineRoleName + "-binding"

	OpenshiftSharedResourceViewRoleBindingName = OpenshiftSharedResourceViewRoleName + "s"
)
