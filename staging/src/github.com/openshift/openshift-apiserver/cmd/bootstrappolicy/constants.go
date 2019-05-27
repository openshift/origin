package bootstrappolicy

// known namespaces
const (
	DefaultOpenShiftSharedResourcesNamespace = "openshift"
	DefaultOpenShiftInfraNamespace           = "openshift-infra"
	DefaultOpenShiftNodeNamespace            = "openshift-node"
)

// users
const (
	DefaultServiceAccountName  = "default"
	BuilderServiceAccountName  = "builder"
	DeployerServiceAccountName = "deployer"

	MasterUnqualifiedUsername     = "openshift-master"
	AggregatorUnqualifiedUsername = "openshift-aggregator"

	MasterUsername      = "system:" + MasterUnqualifiedUsername
	AggregatorUsername  = "system:" + AggregatorUnqualifiedUsername
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
	AuthenticatedGroup      = "system:authenticated"
	AuthenticatedOAuthGroup = "system:authenticated:oauth"
	UnauthenticatedGroup    = "system:unauthenticated"
	ClusterAdminGroup       = "system:cluster-admins"
	ClusterReaderGroup      = "system:cluster-readers"
	MastersGroup            = "system:masters"
	NodesGroup              = "system:nodes"
	NodeAdminsGroup         = "system:node-admins"
)

// Service Account Names that are not controller related
const (
	InfraNodeBootstrapServiceAccountName = "node-bootstrapper"
)

// Roles
const (
	ClusterAdminRoleName            = "cluster-admin"
	SudoerRoleName                  = "sudoer"
	ScopeImpersonationRoleName      = "system:scope-impersonation"
	ClusterReaderRoleName           = "cluster-reader"
	StorageAdminRoleName            = "storage-admin"
	ClusterDebuggerRoleName         = "cluster-debugger"
	AdminRoleName                   = "admin"
	EditRoleName                    = "edit"
	ViewRoleName                    = "view"
	AggregatedAdminRoleName         = "system:openshift:aggregate-to-admin"
	AggregatedEditRoleName          = "system:openshift:aggregate-to-edit"
	AggregatedViewRoleName          = "system:openshift:aggregate-to-view"
	AggregatedClusterReaderRoleName = "system:openshift:aggregate-to-cluster-reader"
	SelfProvisionerRoleName         = "self-provisioner"
	BasicUserRoleName               = "basic-user"
	StatusCheckerRoleName           = "cluster-status"
	SelfAccessReviewerRoleName      = "self-access-reviewer"

	RegistryAdminRoleName  = "registry-admin"
	RegistryViewerRoleName = "registry-viewer"
	RegistryEditorRoleName = "registry-editor"

	TemplateServiceBrokerClientRoleName = "system:openshift:templateservicebroker-client"

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
	SDNReaderRoleName         = "system:sdn-reader"
	SDNManagerRoleName        = "system:sdn-manager"
	OAuthTokenDeleterRoleName = "system:oauth-token-deleter"
	WebHooksRoleName          = "system:webhook"
	DiscoveryRoleName         = "system:openshift:discovery"

	// NodeAdmin has full access to the API provided by the kubelet
	NodeAdminRoleName = "system:node-admin"
	// NodeReader has read access to the metrics and stats provided by the kubelet
	NodeReaderRoleName = "system:node-reader"

	OpenshiftSharedResourceViewRoleName = "shared-resource-viewer"

	NodeBootstrapRoleName    = "system:node-bootstrapper"
	NodeConfigReaderRoleName = "system:node-config-reader"
)

// RoleBindings
const (
	// Legacy roles that must continue to have a plural form
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
	MasterRoleBindingName             = MasterRoleName + "s"
	NodeProxierRoleBindingName        = "system:node-proxier" + "s"
	NodeAdminRoleBindingName          = NodeAdminRoleName + "s"
	SDNReaderRoleBindingName          = SDNReaderRoleName + "s"
	WebHooksRoleBindingName           = WebHooksRoleName + "s"

	OpenshiftSharedResourceViewRoleBindingName = OpenshiftSharedResourceViewRoleName + "s"

	// Bindings
	BuildStrategyDockerRoleBindingName          = BuildStrategyDockerRoleName + "-binding"
	BuildStrategySourceRoleBindingName          = BuildStrategySourceRoleName + "-binding"
	BuildStrategyJenkinsPipelineRoleBindingName = BuildStrategyJenkinsPipelineRoleName + "-binding"
)

// Resources and Subresources
const (
	// Authorization resources
	DockerBuildResource          = "builds/docker"
	OptimizedDockerBuildResource = "builds/optimizeddocker"
	SourceBuildResource          = "builds/source"
	CustomBuildResource          = "builds/custom"
	JenkinsPipelineBuildResource = "builds/jenkinspipeline"

	// These are valid under the "nodes" resource
	NodeMetricsSubresource = "metrics"
	NodeStatsSubresource   = "stats"
	NodeSpecSubresource    = "spec"
	NodeLogSubresource     = "log"
)
