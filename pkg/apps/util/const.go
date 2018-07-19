package util

// DeploymentStatus describes the possible states a deployment can be in.
type DeploymentStatus string

const (

	// TODO: Should move to openshift/api
	// DeploymentStatusNew means the deployment has been accepted but not yet acted upon.
	DeploymentStatusNew DeploymentStatus = "New"
	// DeploymentStatusPending means the deployment been handed over to a deployment strategy,
	// but the strategy has not yet declared the deployment to be running.
	DeploymentStatusPending DeploymentStatus = "Pending"
	// DeploymentStatusRunning means the deployment strategy has reported the deployment as
	// being in-progress.
	DeploymentStatusRunning DeploymentStatus = "Running"
	// DeploymentStatusComplete means the deployment finished without an error.
	DeploymentStatusComplete DeploymentStatus = "Complete"
	// DeploymentStatusFailed means the deployment finished with an error.
	DeploymentStatusFailed DeploymentStatus = "Failed"

	// ReplicationControllerUpdatedReason is added in a deployment config when one of its replication
	// controllers is updated as part of the rollout process.
	ReplicationControllerUpdatedReason = "ReplicationControllerUpdated"
	// FailedRcCreateReason is added in a deployment config when it cannot create a new replication
	// controller.
	FailedRcCreateReason = "ReplicationControllerCreateError"
	// NewReplicationControllerReason is added in a deployment config when it creates a new replication
	// controller.
	NewReplicationControllerReason = "NewReplicationControllerCreated"
	// NewRcAvailableReason is added in a deployment config when its newest replication controller is made
	// available ie. the number of new pods that have passed readiness checks and run for at least
	// minReadySeconds is at least the minimum available pods that need to run for the deployment config.
	NewRcAvailableReason = "NewReplicationControllerAvailable"
	// TimedOutReason is added in a deployment config when its newest replication controller fails to show
	// any progress within the given deadline (progressDeadlineSeconds).
	TimedOutReason = "ProgressDeadlineExceeded"
	// PausedConfigReason is added in a deployment config when it is paused. Lack of progress shouldn't be
	// estimated once a deployment config is paused.
	PausedConfigReason = "DeploymentConfigPaused"
	// CancelledRolloutReason is added in a deployment config when its newest rollout was
	// interrupted by cancellation.
	CancelledRolloutReason = "RolloutCancelled"

	// DeploymentStatusAnnotation is an annotation name used to retrieve the DeploymentPhase of
	// a deployment.
	// Used by CLI and utils:
	// TODO: Should move to library-go?
	DeploymentStatusAnnotation = "openshift.io/deployment.phase"

	// DeployerPodForDeploymentLabel is a label which groups pods related to a
	// Used by utils and lifecycle hooks:
	DeployerPodForDeploymentLabel = "openshift.io/deployer-pod-for.name"

	// DeploymentConfigLabel is the name of a label used to correlate a deployment with the
	DeploymentConfigLabel = "deploymentconfig"

	// DeploymentLabel is the name of a label used to correlate a deployment with the Pod created
	DeploymentLabel = "deployment"

	// MaxDeploymentDurationSeconds represents the maximum duration that a deployment is allowed to run.
	// This is set as the default value for ActiveDeadlineSeconds for the deployer pod.
	// Currently set to 6 hours.
	MaxDeploymentDurationSeconds int64 = 21600

	// DefaultRecreateTimeoutSeconds is the default TimeoutSeconds for RecreateDeploymentStrategyParams.
	// Used by strategies:
	DefaultRecreateTimeoutSeconds int64 = 10 * 60

	// PreHookPodSuffix is the suffix added to all pre hook pods
	PreHookPodSuffix = "hook-pre"
	// MidHookPodSuffix is the suffix added to all mid hook pods
	MidHookPodSuffix = "hook-mid"
	// PostHookPodSuffix is the suffix added to all post hook pods
	PostHookPodSuffix = "hook-post"

	// Used only internally by utils:

	// deploymentStatusReasonAnnotation represents the reason for deployment being in a given state
	// Used for specifying the reason for cancellation or failure of a deployment
	deploymentStatusReasonAnnotation = "openshift.io/deployment.status-reason"
	deploymentCancelledAnnotation    = "openshift.io/deployment.cancelled"
	deploymentCancelledByUser        = "cancelled by the user"

	deploymentIgnorePodAnnotation     = "deploy.openshift.io/deployer-pod.ignore"
	deploymentEncodedConfigAnnotation = "openshift.io/encoded-deployment-config"
	deploymentPodAnnotation           = "openshift.io/deployer-pod.name"
	deploymentAnnotation              = "openshift.io/deployment.name"
	desiredReplicasAnnotation         = "kubectl.kubernetes.io/desired-replicas"
	deploymentReplicasAnnotation      = "openshift.io/deployment.replicas"
	deploymentConfigAnnotation        = "openshift.io/deployment-config.name"
	deploymentVersionAnnotation       = "openshift.io/deployment-config.latest-version"
)
