package util

// TODO: This list needs triage and move to openshift/api and library-go:

var (
	WhitelistEnvVarNames = []string{"BUILD_LOGLEVEL", "GIT_SSL_NO_VERIFY"}

	// DefaultSuccessfulBuildsHistoryLimit is the default number of successful builds to retain
	DefaultSuccessfulBuildsHistoryLimit = int32(5)

	// DefaultFailedBuildsHistoryLimit is the default number of failed builds to retain
	DefaultFailedBuildsHistoryLimit = int32(5)
)

const (
	// BuildAnnotation is an annotation that identifies a Pod as being for a Build
	BuildAnnotation = "openshift.io/build.name"
	// BuildConfigAnnotation is an annotation that identifies the BuildConfig that a Build was created from
	BuildConfigAnnotation = "openshift.io/build-config.name"
	// BuildNumberAnnotation is an annotation whose value is the sequential number for this Build
	BuildNumberAnnotation = "openshift.io/build.number"
	// BuildCloneAnnotation is an annotation whose value is the name of the build this build was cloned from
	BuildCloneAnnotation = "openshift.io/build.clone-of"
	// BuildPodNameAnnotation is an annotation whose value is the name of the pod running this build
	BuildPodNameAnnotation = "openshift.io/build.pod-name"
	// BuildJenkinsStatusJSONAnnotation is an annotation holding the Jenkins status information
	BuildJenkinsStatusJSONAnnotation = "openshift.io/jenkins-status-json"
	// BuildJenkinsLogURLAnnotation is an annotation holding a link to the raw Jenkins build console log
	BuildJenkinsLogURLAnnotation = "openshift.io/jenkins-log-url"
	// BuildJenkinsConsoleLogURLAnnotation is an annotation holding a link to the Jenkins build console log (including Jenkins chrome wrappering)
	BuildJenkinsConsoleLogURLAnnotation = "openshift.io/jenkins-console-log-url"
	// BuildJenkinsBlueOceanLogURLAnnotation is an annotation holding a link to the Jenkins build console log via the Jenkins BlueOcean UI Plugin
	BuildJenkinsBlueOceanLogURLAnnotation = "openshift.io/jenkins-blueocean-log-url"
	// BuildJenkinsBuildURIAnnotation is an annotation holding a link to the Jenkins build
	BuildJenkinsBuildURIAnnotation = "openshift.io/jenkins-build-uri"
	// BuildSourceSecretMatchURIAnnotationPrefix is a prefix for annotations on a Secret which indicate a source URI against which the Secret can be used
	BuildSourceSecretMatchURIAnnotationPrefix = "build.openshift.io/source-secret-match-uri-"
	// BuildLabel is the key of a Pod label whose value is the Name of a Build which is run.
	// NOTE: The value for this label may not contain the entire Build name because it will be
	// truncated to maximum label length.
	BuildLabel = "openshift.io/build.name"
	// BuildRunPolicyLabel represents the start policy used to to start the build.
	BuildRunPolicyLabel = "openshift.io/build.start-policy"
	// AllowedUIDs is an environment variable that contains ranges of UIDs that are allowed in
	// Source builder images
	AllowedUIDs = "ALLOWED_UIDS"
	// DropCapabilities is an environment variable that contains a list of capabilities to drop when
	// executing a Source build
	DropCapabilities = "DROP_CAPS"
	// BuildConfigLabel is the key of a Build label whose value is the ID of a BuildConfig
	// on which the Build is based. NOTE: The value for this label may not contain the entire
	// BuildConfig name because it will be truncated to maximum label length.
	BuildConfigLabel = "openshift.io/build-config.name"
	// BuildConfigLabelDeprecated was used as BuildConfigLabel before adding namespaces.
	// We keep it for backward compatibility.
	BuildConfigLabelDeprecated = "buildconfig"
	// BuildConfigPausedAnnotation is an annotation that marks a BuildConfig as paused.
	// New Builds cannot be instantiated from a paused BuildConfig.
	BuildConfigPausedAnnotation = "openshift.io/build-config.paused"
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
	BuildTriggerCauseManualMsg    = "Manually triggered"
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
	StatusMessagePostCommitHookFailed            = "Build failed because of post commit hook."
)

const (
	// WebHookSecretKey is the key used to identify the value containing the webhook invocation
	// secret within a secret referenced by a webhook trigger.
	WebHookSecretKey = "WebHookSecretKey"

	// CustomBuildStrategyBaseImageKey is the environment variable that indicates the base image to be used when
	// performing a custom build, if needed.
	CustomBuildStrategyBaseImageKey = "OPENSHIFT_CUSTOM_BUILD_BASE_IMAGE"
)
