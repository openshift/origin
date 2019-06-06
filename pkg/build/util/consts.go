package util

// TODO: This list needs triage and move to openshift/api and library-go:

var (
	WhitelistEnvVarNames = []string{"BUILD_LOGLEVEL", "GIT_SSL_NO_VERIFY", "HTTP_PROXY", "HTTPS_PROXY", "LANG", "NO_PROXY"}

	// DefaultSuccessfulBuildsHistoryLimit is the default number of successful builds to retain
	DefaultSuccessfulBuildsHistoryLimit = int32(5)

	// DefaultFailedBuildsHistoryLimit is the default number of failed builds to retain
	DefaultFailedBuildsHistoryLimit = int32(5)
)

const (
	// AllowedUIDs is an environment variable that contains ranges of UIDs that are allowed in
	// Source builder images
	AllowedUIDs = "ALLOWED_UIDS"
	// DropCapabilities is an environment variable that contains a list of capabilities to drop when
	// executing a Source build
	DropCapabilities = "DROP_CAPS"

	// BuildStartedEventReason is the reason associated with the event registered when a build is started (pod is created).
	BuildStartedEventReason = "BuildStarted"
	// BuildStartedEventMessage is the message associated with the event registered when a build is started (pod is created).
	BuildStartedEventMessage = "Build %s/%s is now running"
	// BuildCompletedEventReason is the reason associated with the event registered when build completes successfully.
	BuildCompletedEventReason = "BuildCompleted"
	// BuildCompletedEventMessage is the message associated with the event registered when build completes successfully.
	BuildCompletedEventMessage = "Build %s/%s completed successfully"
	// BuildFailedEventReason is the reason associated with the event registered when build fails.
	BuildFailedEventReason = "BuildFailed"
	// BuildFailedEventMessage is the message associated with the event registered when build fails.
	BuildFailedEventMessage = "Build %s/%s failed"
	// BuildCancelledEventReason is the reason associated with the event registered when build is cancelled.
	BuildCancelledEventReason = "BuildCancelled"
	// BuildCancelledEventMessage is the message associated with the event registered when build is cancelled.
	BuildCancelledEventMessage = "Build %s/%s has been cancelled"
)

const (
	BuildTriggerCauseConfigMsg    = "Build configuration change"
	BuildTriggerCauseImageMsg     = "Image change"
	BuildTriggerCauseGithubMsg    = "GitHub WebHook"
	BuildTriggerCauseGenericMsg   = "Generic WebHook"
	BuildTriggerCauseGitLabMsg    = "GitLab WebHook"
	BuildTriggerCauseBitbucketMsg = "Bitbucket WebHook"
)

const (
	StatusMessageCannotCreateBuildPodSpec        = "Failed to create pod spec."
	StatusMessageCannotCreateBuildPod            = "Failed creating build pod."
	StatusMessageCannotCreateCAConfigMap         = "Failed creating build certificate authority configMap."
	StatusMessageCannotCreateBuildSysConfigMap   = "Failed creating build system config configMap."
	StatusMessageInvalidOutputRef                = "Output image could not be resolved."
	StatusMessageInvalidImageRef                 = "Referenced image could not be resolved."
	StatusMessageBuildPodDeleted                 = "The pod for this build was deleted before the build completed."
	StatusMessageMissingPushSecret               = "Missing push secret."
	StatusMessageCancelledBuild                  = "The build was cancelled by the user."
	StatusMessageBuildPodExists                  = "The pod for this build already exists and is older than the build."
	StatusMessageNoBuildContainerStatus          = "The pod for this build has no container statuses indicating success or failure."
	StatusMessageFailedContainer                 = "The pod for this build has at least one container with a non-zero exit status."
	StatusMessageGenericBuildFailed              = "Generic Build failure - check logs for details."
	StatusMessageOutOfMemoryKilled               = "The build pod was killed due to an out of memory condition."
	StatusMessageUnresolvableEnvironmentVariable = "Unable to resolve build environment variable reference."
	StatusMessageCannotRetrieveServiceAccount    = "Unable to look up the service account secrets for this build."
)

const (
	// WebHookSecretKey is the key used to identify the value containing the webhook invocation
	// secret within a secret referenced by a webhook trigger.
	WebHookSecretKey = "WebHookSecretKey"

	// CustomBuildStrategyBaseImageKey is the environment variable that indicates the base image to be used when
	// performing a custom build, if needed.
	CustomBuildStrategyBaseImageKey = "OPENSHIFT_CUSTOM_BUILD_BASE_IMAGE"

	// RegistryConfKey is the ConfigMap key for the build pod's registry configuration file.
	RegistryConfKey = "registries.conf"

	// SignaturePolicyKey is the ConfigMap key for the build pod's image signature policy file.
	SignaturePolicyKey = "policy.json"

	// ServiceCAKey is the ConfigMap key for the service signing certificate authority mounted into build pods.
	ServiceCAKey = "service-ca.crt"
)
