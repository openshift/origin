package util

const (
	// DeploymentStatusAnnotation is an annotation name used to retrieve the DeploymentPhase of
	// a deployment.
	// Used by CLI and utils:
	// TODO: Should move to library-go?
	DeploymentStatusAnnotation = "openshift.io/deployment.phase"

	// DeployerPodForDeploymentLabel is a label which groups pods related to a
	// Used by utils and lifecycle hooks:
	DeployerPodForDeploymentLabel = "openshift.io/deployer-pod-for.name"

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
	deploymentEncodedConfigAnnotation = "openshift.io/encoded-deployment-config"
	deploymentPodAnnotation           = "openshift.io/deployer-pod.name"
	desiredReplicasAnnotation         = "kubectl.kubernetes.io/desired-replicas"
	deploymentConfigAnnotation        = "openshift.io/deployment-config.name"
	deploymentVersionAnnotation       = "openshift.io/deployment-config.latest-version"
)

// DeploymentStatus describes the possible states a deployment can be in.
type DeploymentStatus string
