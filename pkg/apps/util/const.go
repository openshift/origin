package util

const (
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
	DefaultRollingTimeoutSeconds  int64 = 10 * 60

	// PreHookPodSuffix is the suffix added to all pre hook pods
	PreHookPodSuffix = "hook-pre"
	// MidHookPodSuffix is the suffix added to all mid hook pods
	MidHookPodSuffix = "hook-mid"
	// PostHookPodSuffix is the suffix added to all post hook pods
	PostHookPodSuffix = "hook-post"

	// Used only internally by utils:

	// DeploymentStatusReasonAnnotation represents the reason for deployment being in a given state
	// Used for specifying the reason for cancellation or failure of a deployment
	DeploymentIgnorePodAnnotation = "deploy.openshift.io/deployer-pod.ignore"
	DeploymentReplicasAnnotation  = "openshift.io/deployment.replicas"

	DeploymentFailedUnrelatedDeploymentExists = "unrelated pod with the same name as this deployment is already running"
	DeploymentFailedUnableToCreateDeployerPod = "unable to create deployer pod"
	DeploymentFailedDeployerPodNoLongerExists = "deployer pod no longer exists"

	deploymentCancelledByUser                = "cancelled by the user"
	deploymentCancelledNewerDeploymentExists = "newer deployment was found running"
)
