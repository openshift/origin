package build

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
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
	// DefaultDockerLabelNamespace is the key of a Build label, whose values are build metadata.
	DefaultDockerLabelNamespace = "io.openshift."
	// OriginVersion is an environment variable key that indicates the version of origin that
	// created this build definition.
	OriginVersion = "ORIGIN_VERSION"
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
	// BuildAcceptedAnnotation is an annotation used to update a build that can now be
	// run based on the RunPolicy (e.g. Serial). Updating the build with this annotation
	// forces the build to be processed by the build controller queue without waiting
	// for a resync.
	BuildAcceptedAnnotation = "build.openshift.io/accepted"

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

	// DefaultSuccessfulBuildsHistoryLimit is the default number of successful builds to retain
	// if the buildconfig does not specify a value.  This only applies to buildconfigs created
	// via the new group api resource, not the legacy resource.
	DefaultSuccessfulBuildsHistoryLimit = int32(5)

	// DefaultFailedBuildsHistoryLimit is the default number of failed builds to retain
	// if the buildconfig does not specify a value.  This only applies to buildconfigs created
	// via the new group api resource, not the legacy resource.
	DefaultFailedBuildsHistoryLimit = int32(5)

	// WebHookSecretKey is the key used to identify the value containing the webhook invocation
	// secret within a secret referenced by a webhook trigger.
	WebHookSecretKey = "WebHookSecretKey"
)

var (
	// WhitelistEnvVarNames is a list of environment variable keys that are allowed to be set by the
	// user on the build pod.
	WhitelistEnvVarNames = [2]string{"BUILD_LOGLEVEL", "GIT_SSL_NO_VERIFY"}
)

// +genclient
// +genclient:method=UpdateDetails,verb=update,subresource=details
// +genclient:method=Clone,verb=create,subresource=clone,input=BuildRequest
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the execution and a reference to the Pod which executed the build.
type Build struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec is all the inputs used to execute the build.
	Spec BuildSpec

	// Status is the current status of the build.
	Status BuildStatus
}

// BuildSpec encapsulates all the inputs necessary to represent a build.
type BuildSpec struct {
	CommonSpec

	// TriggeredBy describes which triggers started the most recent update to the
	// build configuration and contains information about those triggers.
	TriggeredBy []BuildTriggerCause
}

// CommonSpec encapsulates all common fields between Build and BuildConfig.
type CommonSpec struct {

	// ServiceAccount is the name of the ServiceAccount to use to run the pod
	// created by this build.
	// The pod will be allowed to use secrets referenced by the ServiceAccount.
	ServiceAccount string

	// Source describes the SCM in use.
	Source BuildSource

	// Revision is the information from the source for a specific repo
	// snapshot.
	// This is optional.
	Revision *SourceRevision

	// Strategy defines how to perform a build.
	Strategy BuildStrategy

	// Output describes the Docker image the Strategy should produce.
	Output BuildOutput

	// Resources computes resource requirements to execute the build.
	Resources kapi.ResourceRequirements

	// PostCommit is a build hook executed after the build output image is
	// committed, before it is pushed to a registry.
	PostCommit BuildPostCommitSpec

	// CompletionDeadlineSeconds is an optional duration in seconds, counted from
	// the time when a build pod gets scheduled in the system, that the build may
	// be active on a node before the system actively tries to terminate the
	// build; value must be positive integer.
	CompletionDeadlineSeconds *int64

	// NodeSelector is a selector which must be true for the build pod to fit on a node
	// If nil, it can be overridden by default build nodeselector values for the cluster.
	// If set to an empty map or a map with any values, default build nodeselector values
	// are ignored.
	NodeSelector map[string]string
}

const (
	BuildTriggerCauseManualMsg    = "Manually triggered"
	BuildTriggerCauseConfigMsg    = "Build configuration change"
	BuildTriggerCauseImageMsg     = "Image change"
	BuildTriggerCauseGithubMsg    = "GitHub WebHook"
	BuildTriggerCauseGenericMsg   = "Generic WebHook"
	BuildTriggerCauseGitLabMsg    = "GitLab WebHook"
	BuildTriggerCauseBitbucketMsg = "Bitbucket WebHook"
)

// BuildTriggerCause holds information about a triggered build. It is used for
// displaying build trigger data for each build and build configuration in oc
// describe. It is also used to describe which triggers led to the most recent
// update in the build configuration.
type BuildTriggerCause struct {
	// Message is used to store a human readable message for why the build was
	// triggered. E.g.: "Manually triggered by user", "Configuration change",etc.
	Message string

	// genericWebHook represents data for a generic webhook that fired a
	// specific build.
	GenericWebHook *GenericWebHookCause

	// GitHubWebHook represents data for a GitHub webhook that fired a specific
	// build.
	GitHubWebHook *GitHubWebHookCause

	// ImageChangeBuild stores information about an imagechange event that
	// triggered a new build.
	ImageChangeBuild *ImageChangeCause

	// GitLabWebHook represents data for a GitLab webhook that fired a specific
	// build.
	GitLabWebHook *GitLabWebHookCause

	// BitbucketWebHook represents data for a Bitbucket webhook that fired a
	// specific build.
	BitbucketWebHook *BitbucketWebHookCause
}

// GenericWebHookCause holds information about a generic WebHook that
// triggered a build.
type GenericWebHookCause struct {
	// Revision is an optional field that stores the git source revision
	// information of the generic webhook trigger when it is available.
	Revision *SourceRevision

	// Secret is the obfuscated webhook secret that triggered a build.
	Secret string
}

// GitHubWebHookCause has information about a GitHub webhook that triggered a
// build.
type GitHubWebHookCause struct {
	// Revision is the git source revision information of the trigger.
	Revision *SourceRevision

	// Secret is the obfuscated webhook secret that triggered a build.
	Secret string
}

// CommonWebHookCause factors out the identical format of these webhook
// causes into struct so we can share it in the specific causes;  it is too late for
// GitHub and Generic but we can leverage this pattern with GitLab and Bitbucket.
type CommonWebHookCause struct {
	// Revision is the git source revision information of the trigger.
	Revision *SourceRevision

	// Secret is the obfuscated webhook secret that triggered a build.
	Secret string
}

// GitLabWebHookCause has information about a GitLab webhook that triggered a
// build.
type GitLabWebHookCause struct {
	CommonWebHookCause
}

// BitbucketWebHookCause has information about a Bitbucket webhook that triggered a
// build.
type BitbucketWebHookCause struct {
	CommonWebHookCause
}

// ImageChangeCause contains information about the image that triggered a
// build.
type ImageChangeCause struct {
	// ImageID is the ID of the image that triggered a a new build.
	ImageID string

	// FromRef contains detailed information about an image that triggered a
	// build
	FromRef *kapi.ObjectReference
}

// BuildStatus contains the status of a build
type BuildStatus struct {
	// Phase is the point in the build lifecycle. Possible values are
	// "New", "Pending", "Running", "Complete", "Failed", "Error", and "Cancelled".
	Phase BuildPhase

	// Cancelled describes if a cancel event was triggered for the build.
	Cancelled bool

	// Reason is a brief CamelCase string that describes any failure and is meant for machine parsing and tidy display in the CLI.
	Reason StatusReason

	// Message is a human-readable message indicating details about why the build has this status.
	Message string

	// StartTimestamp is a timestamp representing the server time when this Build started
	// running in a Pod.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *metav1.Time

	// CompletionTimestamp is a timestamp representing the server time when this Build was
	// finished, whether that build failed or succeeded.  It reflects the time at which
	// the Pod running the Build terminated.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTimestamp *metav1.Time

	// Duration contains time.Duration object describing build time.
	Duration time.Duration

	// OutputDockerImageReference contains a reference to the Docker image that
	// will be built by this build. It's value is computed from
	// Build.Spec.Output.To, and should include the registry address, so that
	// it can be used to push and pull the image.
	OutputDockerImageReference string

	// Config is an ObjectReference to the BuildConfig this Build is based on.
	Config *kapi.ObjectReference

	// Output describes the Docker image the build has produced.
	Output BuildStatusOutput

	// Stages contains details about each stage that occurs during the build
	// including start time, duration (in milliseconds), and the steps that
	// occured within each stage.
	Stages []StageInfo

	// LogSnippet is the last few lines of the build log.  This value is only set for builds that failed.
	LogSnippet string
}

// StageInfo contains details about a build stage.
type StageInfo struct {
	// Name is a unique identifier for each build stage that occurs.
	Name StageName

	// StartTime is a timestamp representing the server time when this Stage started.
	// It is represented in RFC3339 form and is in UTC.
	StartTime metav1.Time

	// DurationMilliseconds identifies how long the stage took
	// to complete in milliseconds.
	// Note: the duration of a stage can exceed the sum of the duration of the steps within
	// the stage as not all actions are accounted for in explicit build steps.
	DurationMilliseconds int64

	// Steps contains details about each step that occurs during a build stage
	// including start time and duration in milliseconds.
	Steps []StepInfo
}

// StageName is the identifier for each build stage.
type StageName string

// Valid values for StageName
const (
	// StageFetchInputs fetches any inputs such as source code.
	StageFetchInputs StageName = "FetchInputs"

	// StagePullImages pulls any images that are needed such as
	// base images or input images.
	StagePullImages StageName = "PullImages"

	// StageBuild performs the steps necessary to build the image.
	StageBuild StageName = "Build"

	// StagePostCommit executes any post commit steps.
	StagePostCommit StageName = "PostCommit"

	// StagePushImage pushes the image to the node.
	StagePushImage StageName = "PushImage"
)

// StepInfo contains details about a build step.
type StepInfo struct {
	// Name is a unique identifier for each build step.
	Name StepName

	// StartTime is a timestamp representing the server time when this Step started.
	// it is represented in RFC3339 form and is in UTC.
	StartTime metav1.Time

	// DurationMilliseconds identifies how long the step took
	// to complete in milliseconds.
	DurationMilliseconds int64
}

// StepName is a unique identifier for each build step.
type StepName string

// Valid values for StepName
const (
	// StepExecPostCommitHook executes the buildconfigs post commit hook.
	StepExecPostCommitHook StepName = "RunPostCommitHook"

	// StepFetchGitSource fetches the source code for the build.
	StepFetchGitSource StepName = "FetchGitSource"

	// StepPullBaseImage pulls the base image for the build.
	StepPullBaseImage StepName = "PullBaseImage"

	// StepPullInputImage pulls the input image for the build.
	StepPullInputImage StepName = "PullInputImage"

	// StepPushImage pushed the image to the registry.
	StepPushImage StepName = "PushImage"

	// StepPushDockerImage pushes the docker image to the registry.
	StepPushDockerImage StepName = "PushDockerImage"

	//StepDockerBuild performs the docker build
	StepDockerBuild StepName = "DockerBuild"
)

// BuildPhase represents the status of a build at a point in time.
type BuildPhase string

// Valid values for BuildPhase.
const (
	// BuildPhaseNew is automatically assigned to a newly created build.
	BuildPhaseNew BuildPhase = "New"

	// BuildPhasePending indicates that a pod name has been assigned and a build is
	// about to start running.
	BuildPhasePending BuildPhase = "Pending"

	// BuildPhaseRunning indicates that a pod has been created and a build is running.
	BuildPhaseRunning BuildPhase = "Running"

	// BuildPhaseComplete indicates that a build has been successful.
	BuildPhaseComplete BuildPhase = "Complete"

	// BuildPhaseFailed indicates that a build has executed and failed.
	BuildPhaseFailed BuildPhase = "Failed"

	// BuildPhaseError indicates that an error prevented the build from executing.
	BuildPhaseError BuildPhase = "Error"

	// BuildPhaseCancelled indicates that a running/pending build was stopped from executing.
	BuildPhaseCancelled BuildPhase = "Cancelled"
)

// StatusReason is a brief CamelCase string that describes a temporary or
// permanent build error condition, meant for machine parsing and tidy display
// in the CLI.
type StatusReason string

// These are the valid reasons of build statuses.
const (
	// StatusReasonError is a generic reason for a build error condition.
	StatusReasonError StatusReason = "Error" //note/fyi:  not used currently, api or v1

	// StatusReasonCannotCreateBuildPodSpec is an error condition when the build
	// strategy cannot create a build pod spec.
	StatusReasonCannotCreateBuildPodSpec StatusReason = "CannotCreateBuildPodSpec"

	// StatusReasonCannotCreateBuildPod is an error condition when a build pod
	// cannot be created.
	StatusReasonCannotCreateBuildPod StatusReason = "CannotCreateBuildPod"

	// StatusReasonInvalidOutputReference is an error condition when the build
	// output is an invalid reference.
	StatusReasonInvalidOutputReference StatusReason = "InvalidOutputReference"

	// StatusReasonInvalidImageReference is an error condition when the build
	// references an invalid image.
	StatusReasonInvalidImageReference StatusReason = "InvalidImageReference"

	// StatusReasonCancelBuildFailed is an error condition when cancelling a build
	// fails.
	StatusReasonCancelBuildFailed StatusReason = "CancelBuildFailed" // note/fyi: not used currently, api or v1

	// StatusReasonBuildPodDeleted is an error condition when the build pod is
	// deleted before build completion.
	StatusReasonBuildPodDeleted StatusReason = "BuildPodDeleted"

	// StatusReasonExceededRetryTimeout is an error condition when the build has
	// not completed and retrying the build times out.
	StatusReasonExceededRetryTimeout StatusReason = "ExceededRetryTimeout" // note/fyi: not used currently, api or v1

	// StatusReasonMissingPushSecret indicates that the build is missing required
	// secret for pushing the output image.
	// The build will stay in the pending state until the secret is created, or the build times out.
	StatusReasonMissingPushSecret StatusReason = "MissingPushSecret"

	// StatusReasonPostCommitHookFailed indicates the post-commit hook failed.
	StatusReasonPostCommitHookFailed StatusReason = "PostCommitHookFailed"

	// StatusReasonPushImageToRegistryFailed indicates that an image failed to be
	// pushed to the registry.
	StatusReasonPushImageToRegistryFailed StatusReason = "PushImageToRegistryFailed"

	// StatusReasonPullBuilderImageFailed indicates that we failed to pull the
	// builder image.
	StatusReasonPullBuilderImageFailed StatusReason = "PullBuilderImageFailed"

	// StatusReasonFetchSourceFailed indicates that fetching the source of the
	// build has failed.
	StatusReasonFetchSourceFailed StatusReason = "FetchSourceFailed"

	// StatusReasonInvalidContextDirectory indicates that the supplied
	// contextDir does not exist
	StatusReasonInvalidContextDirectory StatusReason = "InvalidContextDirectory"

	// StatusReasonCancelledBuild indicates that the build was cancelled by the
	// user.
	StatusReasonCancelledBuild StatusReason = "CancelledBuild"

	// StatusReasonDockerBuildFailed indicates that the docker build strategy has
	// failed.
	StatusReasonDockerBuildFailed StatusReason = "DockerBuildFailed"

	// StatusReasonBuildPodExists indicates that the build tried to create a
	// build pod but one was already present.
	StatusReasonBuildPodExists StatusReason = "BuildPodExists"

	// StatusReasonNoBuildContainerStatus indicates that the build failed because the
	// the build pod has no container statuses.
	StatusReasonNoBuildContainerStatus StatusReason = "NoBuildContainerStatus"

	// StatusReasonFailedContainer indicates that the pod for the build has at least
	// one container with a non-zero exit status.
	StatusReasonFailedContainer StatusReason = "FailedContainer"

	// StatusReasonUnresolvableEnvironmentVariable indicates that an error occurred processing
	// the supplied options for environment variables in the build strategy environment
	StatusReasonUnresolvableEnvironmentVariable StatusReason = "UnresolvableEnvironmentVariable"

	// StatusReasonGenericBuildFailed is the reason associated with a broad
	// range of build failures.
	StatusReasonGenericBuildFailed StatusReason = "GenericBuildFailed"

	// StatusCannotRetrieveServiceAccount is the reason associated with a failure
	// to look up the service account associated with the BuildConfig.
	StatusReasonCannotRetrieveServiceAccount StatusReason = "CannotRetrieveServiceAccount"
)

// NOTE: These messages might change.
const (
	StatusMessageCannotCreateBuildPodSpec        = "Failed to create pod spec."
	StatusMessageCannotCreateBuildPod            = "Failed creating build pod."
	StatusMessageInvalidOutputRef                = "Output image could not be resolved."
	StatusMessageInvalidImageRef                 = "Referenced image could not be resolved."
	StatusMessageCancelBuildFailed               = "Failed to cancel build."
	StatusMessageBuildPodDeleted                 = "The pod for this build was deleted before the build completed."
	StatusMessageExceededRetryTimeout            = "Build did not complete and retrying timed out."
	StatusMessageMissingPushSecret               = "Missing push secret."
	StatusMessagePostCommitHookFailed            = "Build failed because of post commit hook."
	StatusMessagePushImageToRegistryFailed       = "Failed to push the image to the registry."
	StatusMessagePullBuilderImageFailed          = "Failed pulling builder image."
	StatusMessageFetchSourceFailed               = "Failed to fetch the input source."
	StatusMessageInvalidContextDirectory         = "The supplied context directory does not exist."
	StatusMessageCancelledBuild                  = "The build was cancelled by the user."
	StatusMessageDockerBuildFailed               = "Docker build strategy has failed."
	StatusMessageBuildPodExists                  = "The pod for this build already exists and is older than the build."
	StatusMessageNoBuildContainerStatus          = "The pod for this build has no container statuses indicating success or failure."
	StatusMessageFailedContainer                 = "The pod for this build has at least one container with a non-zero exit status."
	StatusMessageGenericBuildFailed              = "Generic Build failure - check logs for details."
	StatusMessageUnresolvableEnvironmentVariable = "Unable to resolve build environment variable reference."
	StatusMessageCannotRetrieveServiceAccount    = "Unable to look up the service account secrets for this build."
)

// BuildStatusOutput contains the status of the built image.
type BuildStatusOutput struct {
	// To describes the status of the built image being pushed to a registry.
	To *BuildStatusOutputTo
}

// BuildStatusOutputTo describes the status of the built image with regards to
// image registry to which it was supposed to be pushed.
type BuildStatusOutputTo struct {
	// ImageDigest is the digest of the built Docker image. The digest uniquely
	// identifies the image in the registry to which it was pushed.
	//
	// Please note that this field may not always be set even if the push
	// completes successfully - e.g. when the registry returns no digest or
	// returns it in a format that the builder doesn't understand.
	ImageDigest string
}

// BuildSource is the input used for the build.
type BuildSource struct {
	// Binary builds accept a binary as their input. The binary is generally assumed to be a tar,
	// gzipped tar, or zip file depending on the strategy. For Docker builds, this is the build
	// context and an optional Dockerfile may be specified to override any Dockerfile in the
	// build context. For Source builds, this is assumed to be an archive as described above. For
	// Source and Docker builds, if binary.asFile is set the build will receive a directory with
	// a single file. contextDir may be used when an archive is provided. Custom builds will
	// receive this binary as input on STDIN.
	Binary *BinaryBuildSource

	// Dockerfile is the raw contents of a Dockerfile which should be built. When this option is
	// specified, the FROM may be modified based on your strategy base image and additional ENV
	// stanzas from your strategy environment will be added after the FROM, but before the rest
	// of your Dockerfile stanzas. The Dockerfile source type may be used with other options like
	// git - in those cases the Git repo will have any innate Dockerfile replaced in the context
	// dir.
	Dockerfile *string

	// Git contains optional information about git build source
	Git *GitBuildSource

	// Images describes a set of images to be used to provide source for the build
	Images []ImageSource

	// ContextDir specifies the sub-directory where the source code for the application exists.
	// This allows to have buildable sources in directory other than root of
	// repository.
	ContextDir string

	// SourceSecret is the name of a Secret that would be used for setting
	// up the authentication for cloning private repository.
	// The secret contains valid credentials for remote repository, where the
	// data's key represent the authentication method to be used and value is
	// the base64 encoded credentials. Supported auth methods are: ssh-privatekey.
	// TODO: This needs to move under the GitBuildSource struct since it's only
	// used for git authentication
	SourceSecret *kapi.LocalObjectReference

	// Secrets represents a list of secrets and their destinations that will
	// be used only for the build.
	Secrets []SecretBuildSource
}

// ImageSource is used to describe build source that will be extracted from an image or used during a
// multi stage build. A reference of type ImageStreamTag, ImageStreamImage or DockerImage may be used.
// A pull secret can be specified to pull the image from an external registry or override the default
// service account secret if pulling from the internal registry. Image sources can either be used to
// extract content from an image and place it into the build context along with the repository source,
// or used directly during a multi-stage Docker build to allow content to be copied without overwriting
// the contents of the repository source (see the 'paths' and 'as' fields).
type ImageSource struct {
	// from is a reference to an ImageStreamTag, ImageStreamImage, or DockerImage to
	// copy source from.
	From kapi.ObjectReference

	// A list of image names that this source will be used in place of during a multi-stage Docker image
	// build. For instance, a Dockerfile that uses "COPY --from=nginx:latest" will first check for an image
	// source that has "nginx:latest" in this field before attempting to pull directly. If the Dockerfile
	// does not reference an image source it is ignored. This field and paths may both be set, in which case
	// the contents will be used twice.
	// +optional
	As []string

	// paths is a list of source and destination paths to copy from the image. This content will be copied
	// into the build context prior to starting the build. If no paths are set, the build context will
	// not be altered.
	// +optional
	Paths []ImageSourcePath

	// pullSecret is a reference to a secret to be used to pull the image from a registry
	// If the image is pulled from the OpenShift registry, this field does not need to be set.
	PullSecret *kapi.LocalObjectReference
}

// ImageSourcePath describes a path to be copied from a source image and its destination within the build directory.
type ImageSourcePath struct {
	// SourcePath is the absolute path of the file or directory inside the image to
	// copy to the build directory.  If the source path ends in /. then the content of
	// the directory will be copied, but the directory itself will not be created at the
	// destination.
	SourcePath string

	// DestinationDir is the relative directory within the build directory
	// where files copied from the image are placed.
	DestinationDir string
}

// SecretBuildSource describes a secret and its destination directory that will be
// used only at the build time. The content of the secret referenced here will
// be copied into the destination directory instead of mounting.
type SecretBuildSource struct {
	// Secret is a reference to an existing secret that you want to use in your
	// build.
	Secret kapi.LocalObjectReference

	// DestinationDir is the directory where the files from the secret should be
	// available for the build time.
	// For the Source build strategy, these will be injected into a container
	// where the assemble script runs. Later, when the script finishes, all files
	// injected will be truncated to zero length.
	// For the Docker build strategy, these will be copied into the build
	// directory, where the Dockerfile is located, so users can ADD or COPY them
	// during docker build.
	DestinationDir string
}

type BinaryBuildSource struct {
	// AsFile indicates that the provided binary input should be considered a single file
	// within the build input. For example, specifying "webapp.war" would place the provided
	// binary as `/webapp.war` for the builder. If left empty, the Docker and Source build
	// strategies assume this file is a zip, tar, or tar.gz file and extract it as the source.
	// The custom strategy receives this binary as standard input. This filename may not
	// contain slashes or be '..' or '.'.
	AsFile string
}

// SourceRevision is the revision or commit information from the source for the build
type SourceRevision struct {
	// Git contains information about git-based build source
	Git *GitSourceRevision
}

// GitSourceRevision is the commit information from a git source for a build
type GitSourceRevision struct {
	// Commit is the commit hash identifying a specific commit
	Commit string

	// Author is the author of a specific commit
	Author SourceControlUser

	// Committer is the committer of a specific commit
	Committer SourceControlUser

	// Message is the description of a specific commit
	Message string
}

// ProxyConfig defines what proxies to use for an operation
type ProxyConfig struct {
	// HTTPProxy is a proxy used to reach the git repository over http
	HTTPProxy *string

	// HTTPSProxy is a proxy used to reach the git repository over https
	HTTPSProxy *string

	// NoProxy is the list of domains for which the proxy should not be used
	NoProxy *string
}

// GitBuildSource defines the parameters of a Git SCM
type GitBuildSource struct {
	// URI points to the source that will be built. The structure of the source
	// will depend on the type of build to run
	URI string

	// Ref is the branch/tag/ref to build.
	Ref string

	// ProxyConfig defines the proxies to use for the git clone operation
	ProxyConfig
}

// SourceControlUser defines the identity of a user of source control
type SourceControlUser struct {
	// Name of the source control user
	Name string

	// Email of the source control user
	Email string
}

// BuildStrategy contains the details of how to perform a build.
type BuildStrategy struct {
	// DockerStrategy holds the parameters to the Docker build strategy.
	DockerStrategy *DockerBuildStrategy

	// SourceStrategy holds the parameters to the Source build strategy.
	SourceStrategy *SourceBuildStrategy

	// CustomStrategy holds the parameters to the Custom build strategy
	CustomStrategy *CustomBuildStrategy

	// JenkinsPipelineStrategy holds the parameters to the Jenkins Pipeline build strategy.
	JenkinsPipelineStrategy *JenkinsPipelineBuildStrategy
}

// BuildStrategyType describes a particular way of performing a build.
type BuildStrategyType string

const (
	// CustomBuildStrategyBaseImageKey is the environment variable that indicates the base image to be used when
	// performing a custom build, if needed.
	CustomBuildStrategyBaseImageKey = "OPENSHIFT_CUSTOM_BUILD_BASE_IMAGE"
)

// CustomBuildStrategy defines input parameters specific to Custom build.
type CustomBuildStrategy struct {
	// From is reference to an DockerImage, ImageStream, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference

	// Env contains additional environment variables you want to pass into a builder container.
	Env []kapi.EnvVar

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool

	// ForcePull describes if the controller should configure the build pod to always pull the images
	// for the builder or only pull if it is not present locally
	ForcePull bool

	// Secrets is a list of additional secrets that will be included in the custom build pod
	Secrets []SecretSpec

	// BuildAPIVersion is the requested API version for the Build object serialized and passed to the custom builder
	BuildAPIVersion string
}

// ImageOptimizationPolicy describes what optimizations the builder can perform when building images.
type ImageOptimizationPolicy string

const (
	// ImageOptimizationNone will generate a canonical Docker image as produced by the
	// `docker build` command.
	ImageOptimizationNone ImageOptimizationPolicy = "None"

	// ImageOptimizationSkipLayers is an experimental policy and will avoid creating
	// unique layers for each dockerfile line, resulting in smaller images and saving time
	// during creation. Some Dockerfile syntax is not fully supported - content added to
	// a VOLUME by an earlier layer may have incorrect uid, gid, and filesystem permissions.
	// If an unsupported setting is detected, the build will fail.
	ImageOptimizationSkipLayers ImageOptimizationPolicy = "SkipLayers"

	// ImageOptimizationSkipLayersAndWarn is the same as SkipLayers, but will only
	// warn to the build output instead of failing when unsupported syntax is detected. This
	// policy is experimental.
	ImageOptimizationSkipLayersAndWarn ImageOptimizationPolicy = "SkipLayersAndWarn"
)

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// From is reference to an DockerImage, ImageStream, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	// the resulting image will be used in the FROM line of the Dockerfile for this build.
	From *kapi.ObjectReference

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference

	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool

	// Env contains additional environment variables you want to pass into a builder container.
	Env []kapi.EnvVar

	// Args contains any build arguments that are to be passed to Docker.  See
	// https://docs.docker.com/engine/reference/builder/#/arg for more details
	BuildArgs []kapi.EnvVar

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool

	// DockerfilePath is the path of the Dockerfile that will be used to build the Docker image,
	// relative to the root of the context (contextDir).
	DockerfilePath string

	// ImageOptimizationPolicy describes what optimizations the system can use when building images
	// to reduce the final size or time spent building the image. The default policy is 'None' which
	// means the final build image will be equivalent to an image created by the Docker build API.
	// The experimental policy 'SkipLayerCache' will avoid commiting new layers in between each
	// image step, and will fail if the Dockerfile cannot provide compatibility with the 'None'
	// policy. An additional experimental policy 'SkipLayerCacheAndWarn' is the same as
	// 'SkipLayerCache' but simply warns if compatibility cannot be preserved.
	ImageOptimizationPolicy *ImageOptimizationPolicy
}

// SourceBuildStrategy defines input parameters specific to an Source build.
type SourceBuildStrategy struct {
	// From is reference to an DockerImage, ImageStream, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference

	// Env contains additional environment variables you want to pass into a builder container.
	Env []kapi.EnvVar

	// Scripts is the location of Source scripts
	Scripts string

	// Incremental flag forces the Source build to do incremental builds if true.
	Incremental *bool

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool
}

// JenkinsPipelineStrategy holds parameters specific to a Jenkins Pipeline build.
type JenkinsPipelineBuildStrategy struct {
	// JenkinsfilePath is the optional path of the Jenkinsfile that will be used to configure the pipeline
	// relative to the root of the context (contextDir). If both JenkinsfilePath & Jenkinsfile are
	// both not specified, this defaults to Jenkinsfile in the root of the specified contextDir.
	JenkinsfilePath string

	// Jenkinsfile defines the optional raw contents of a Jenkinsfile which defines a Jenkins pipeline build.
	Jenkinsfile string

	// Env contains additional environment variables you want to pass into a build pipeline.
	Env []kapi.EnvVar
}

// A BuildPostCommitSpec holds a build post commit hook specification. The hook
// executes a command in a temporary container running the build output image,
// immediately after the last layer of the image is committed and before the
// image is pushed to a registry. The command is executed with the current
// working directory ($PWD) set to the image's WORKDIR.
//
// The build will be marked as failed if the hook execution fails. It will fail
// if the script or command return a non-zero exit code, or if there is any
// other error related to starting the temporary container.
//
// There are five different ways to configure the hook. As an example, all forms
// below are equivalent and will execute `rake test --verbose`.
//
// 1. Shell script:
//
// 	BuildPostCommitSpec{
// 		Script: "rake test --verbose",
// 	}
//
// The above is a convenient form which is equivalent to:
//
// 	BuildPostCommitSpec{
// 		Command: []string{"/bin/sh", "-ic"},
// 		Args: []string{"rake test --verbose"},
// 	}
//
// 2. Command as the image entrypoint:
//
// 	BuildPostCommitSpec{
// 		Command: []string{"rake", "test", "--verbose"},
// 	}
//
// Command overrides the image entrypoint in the exec form, as documented in
// Docker: https://docs.docker.com/engine/reference/builder/#entrypoint.
//
// 3. Pass arguments to the default entrypoint:
//
// 	BuildPostCommitSpec{
// 		Args: []string{"rake", "test", "--verbose"},
// 	}
//
// This form is only useful if the image entrypoint can handle arguments.
//
// 4. Shell script with arguments:
//
// 	BuildPostCommitSpec{
// 		Script: "rake test $1",
// 		Args: []string{"--verbose"},
// 	}
//
// This form is useful if you need to pass arguments that would otherwise be
// hard to quote properly in the shell script. In the script, $0 will be
// "/bin/sh" and $1, $2, etc, are the positional arguments from Args.
//
// 5. Command with arguments:
//
// 	BuildPostCommitSpec{
// 		Command: []string{"rake", "test"},
// 		Args: []string{"--verbose"},
// 	}
//
// This form is equivalent to appending the arguments to the Command slice.
//
// It is invalid to provide both Script and Command simultaneously. If none of
// the fields are specified, the hook is not executed.
type BuildPostCommitSpec struct {
	// Command is the command to run. It may not be specified with Script.
	// This might be needed if the image doesn't have `/bin/sh`, or if you
	// do not want to use a shell. In all other cases, using Script might be
	// more convenient.
	Command []string
	// Args is a list of arguments that are provided to either Command,
	// Script or the Docker image's default entrypoint. The arguments are
	// placed immediately after the command to be run.
	Args []string
	// Script is a shell script to be run with `/bin/sh -ic`. It may not be
	// specified with Command. Use Script when a shell script is appropriate
	// to execute the post build hook, for example for running unit tests
	// with `rake test`. If you need control over the image entrypoint, or
	// if the image does not have `/bin/sh`, use Command and/or Args.
	// The `-i` flag is needed to support CentOS and RHEL images that use
	// Software Collections (SCL), in order to have the appropriate
	// collections enabled in the shell. E.g., in the Ruby image, this is
	// necessary to make `ruby`, `bundle` and other binaries available in
	// the PATH.
	Script string
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// To defines an optional location to push the output of this build to.
	// Kind must be one of 'ImageStreamTag' or 'DockerImage'.
	// This value will be used to look up a Docker image repository to push to.
	// In the case of an ImageStreamTag, the ImageStreamTag will be looked for in the namespace of
	// the build unless Namespace is specified.
	To *kapi.ObjectReference

	// PushSecret is the name of a Secret that would be used for setting
	// up the authentication for executing the Docker push to authentication
	// enabled Docker Registry (or Docker Hub).
	PushSecret *kapi.LocalObjectReference

	// ImageLabels define a list of labels that are applied to the resulting image. If there
	// are multiple labels with the same name then the last one in the list is used.
	ImageLabels []ImageLabel
}

// ImageLabel represents a label applied to the resulting image.
type ImageLabel struct {
	// Name defines the name of the label. It must have non-zero length.
	Name string

	// Value defines the literal value of the label.
	Value string
}

// +genclient
// +genclient:method=Instantiate,verb=create,subresource=instantiate,input=BuildRequest,result=Build
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildConfig is a template which can be used to create new builds.
type BuildConfig struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec holds all the input necessary to produce a new build, and the conditions when
	// to trigger them.
	Spec BuildConfigSpec
	// Status holds any relevant information about a build config
	Status BuildConfigStatus
}

// BuildConfigSpec describes when and how builds are created
type BuildConfigSpec struct {
	// Triggers determine how new Builds can be launched from a BuildConfig. If
	// no triggers are defined, a new build can only occur as a result of an
	// explicit client build creation.
	Triggers []BuildTriggerPolicy

	// RunPolicy describes how the new build created from this build
	// configuration will be scheduled for execution.
	// This is optional, if not specified we default to "Serial".
	RunPolicy BuildRunPolicy

	// CommonSpec is the desired build specification
	CommonSpec

	// SuccessfulBuildsHistoryLimit is the number of old successful builds to retain.
	// This field is a pointer to allow for differentiation between an explicit zero and not specified.
	SuccessfulBuildsHistoryLimit *int32

	// FailedBuildsHistoryLimit is the number of old failed builds to retain.
	// This field is a pointer to allow for differentiation between an explicit zero and not specified.
	FailedBuildsHistoryLimit *int32
}

// BuildRunPolicy defines the behaviour of how the new builds are executed
// from the existing build configuration.
type BuildRunPolicy string

const (
	// BuildRunPolicyParallel schedules new builds immediately after they are
	// created. Builds will be executed in parallel.
	BuildRunPolicyParallel BuildRunPolicy = "Parallel"

	// BuildRunPolicySerial schedules new builds to execute in a sequence as
	// they are created. Every build gets queued up and will execute when the
	// previous build completes. This is the default policy.
	BuildRunPolicySerial BuildRunPolicy = "Serial"

	// BuildRunPolicySerialLatestOnly schedules only the latest build to execute,
	// cancelling all the previously queued build.
	BuildRunPolicySerialLatestOnly BuildRunPolicy = "SerialLatestOnly"
)

// BuildConfigStatus contains current state of the build config object.
type BuildConfigStatus struct {
	// LastVersion is used to inform about number of last triggered build.
	LastVersion int64
}

// SecretLocalReference contains information that points to the local secret being used
type SecretLocalReference struct {
	// Name is the name of the resource in the same namespace being referenced
	Name string
}

// WebHookTrigger is a trigger that gets invoked using a webhook type of post
type WebHookTrigger struct {
	// Secret used to validate requests.
	// Deprecated: use SecretReference instead.
	Secret string

	// AllowEnv determines whether the webhook can set environment variables; can only
	// be set to true for GenericWebHook
	AllowEnv bool

	// SecretReference is a reference to a secret in the same namespace,
	// containing the value to be validated when the webhook is invoked.
	// The secret being referenced must contain a key named "WebHookSecretKey", the value
	// of which will be checked against the value supplied in the webhook invocation.
	SecretReference *SecretLocalReference
}

// ImageChangeTrigger allows builds to be triggered when an ImageStream changes
type ImageChangeTrigger struct {
	// LastTriggeredImageID is used internally by the ImageChangeController to save last
	// used image ID for build
	LastTriggeredImageID string

	// From is a reference to an ImageStreamTag that will trigger a build when updated
	// It is optional. If no From is specified, the From image from the build strategy
	// will be used. Only one ImageChangeTrigger with an empty From reference is allowed in
	// a build configuration.
	From *kapi.ObjectReference
}

// BuildTriggerPolicy describes a policy for a single trigger that results in a new Build.
type BuildTriggerPolicy struct {
	// Type is the type of build trigger
	Type BuildTriggerType

	// GitHubWebHook contains the parameters for a GitHub webhook type of trigger
	GitHubWebHook *WebHookTrigger

	// GenericWebHook contains the parameters for a Generic webhook type of trigger
	GenericWebHook *WebHookTrigger

	// ImageChange contains parameters for an ImageChange type of trigger
	ImageChange *ImageChangeTrigger

	// GitLabWebHook contains the parameters for a GitLab webhook type of trigger
	GitLabWebHook *WebHookTrigger

	// BitbucketWebHook contains the parameters for a Bitbucket webhook type of
	// trigger
	BitbucketWebHook *WebHookTrigger
}

// BuildTriggerType refers to a specific BuildTriggerPolicy implementation.
type BuildTriggerType string

//NOTE: Adding a new trigger type requires adding the type to KnownTriggerTypes
var KnownTriggerTypes = sets.NewString(
	string(GitHubWebHookBuildTriggerType),
	string(GenericWebHookBuildTriggerType),
	string(ImageChangeBuildTriggerType),
	string(ConfigChangeBuildTriggerType),
	string(GitLabWebHookBuildTriggerType),
	string(BitbucketWebHookBuildTriggerType),
)

const (
	// GitHubWebHookBuildTriggerType represents a trigger that launches builds on
	// GitHub webhook invocations
	GitHubWebHookBuildTriggerType           BuildTriggerType = "GitHub"
	GitHubWebHookBuildTriggerTypeDeprecated BuildTriggerType = "github"

	// GenericWebHookBuildTriggerType represents a trigger that launches builds on
	// generic webhook invocations
	GenericWebHookBuildTriggerType           BuildTriggerType = "Generic"
	GenericWebHookBuildTriggerTypeDeprecated BuildTriggerType = "generic"

	// GitLabWebHookBuildTriggerType represents a trigger that launches builds on
	// GitLab webhook invocations
	GitLabWebHookBuildTriggerType BuildTriggerType = "GitLab"

	// BitbucketWebHookBuildTriggerType represents a trigger that launches builds on
	// Bitbucket webhook invocations
	BitbucketWebHookBuildTriggerType BuildTriggerType = "Bitbucket"

	// ImageChangeBuildTriggerType represents a trigger that launches builds on
	// availability of a new version of an image
	ImageChangeBuildTriggerType           BuildTriggerType = "ImageChange"
	ImageChangeBuildTriggerTypeDeprecated BuildTriggerType = "imageChange"

	// ConfigChangeBuildTriggerType will trigger a build on an initial build config creation
	// WARNING: In the future the behavior will change to trigger a build on any config change
	ConfigChangeBuildTriggerType BuildTriggerType = "ConfigChange"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildList is a collection of Builds.
type BuildList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items is a list of builds
	Items []Build
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items is a list of build configs
	Items []BuildConfig
}

// GenericWebHookEvent is the payload expected for a generic webhook post
type GenericWebHookEvent struct {
	// Git is the git information, if any.
	Git *GitInfo

	// Env contains additional environment variables you want to pass into a builder container.
	// ValueFrom is not supported.
	Env []kapi.EnvVar

	// DockerStrategyOptions contains additional docker-strategy specific options for the build
	DockerStrategyOptions *DockerStrategyOptions
}

// GitInfo is the aggregated git information for a generic webhook post
type GitInfo struct {
	GitBuildSource
	GitSourceRevision

	// Refs is a list of GitRefs for the provided repo - generally sent
	// when used from a post-receive hook. This field is optional and is
	// used when sending multiple refs
	// +k8s:conversion-gen=false
	Refs []GitRefInfo
}

// GitRefInfo is a single ref
type GitRefInfo struct {
	GitBuildSource
	GitSourceRevision
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildLog is the (unused) resource associated with the build log redirector
type BuildLog struct {
	metav1.TypeMeta
}

// DockerStrategyOptions contains extra strategy options for Docker builds
type DockerStrategyOptions struct {
	// Args contains any build arguments that are to be passed to Docker.  See
	// https://docs.docker.com/engine/reference/builder/#/arg for more details
	BuildArgs []kapi.EnvVar

	// NoCache overrides the docker-strategy noCache option in the build config
	NoCache *bool
}

// SourceStrategyOptions contains extra strategy options for Source builds
type SourceStrategyOptions struct {
	// Incremental overrides the source-strategy incremental option in the build config
	Incremental *bool
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildRequest is the resource used to pass parameters to build generator
type BuildRequest struct {
	metav1.TypeMeta
	// TODO: build request should allow name generation via Name and GenerateName, build config
	// name should be provided as a separate field
	metav1.ObjectMeta

	// Revision is the information from the source for a specific repo snapshot.
	Revision *SourceRevision

	// TriggeredByImage is the Image that triggered this build.
	TriggeredByImage *kapi.ObjectReference

	// From is the reference to the ImageStreamTag that triggered the build.
	From *kapi.ObjectReference

	// Binary indicates a request to build from a binary provided to the builder
	Binary *BinaryBuildSource

	// LastVersion (optional) is the LastVersion of the BuildConfig that was used
	// to generate the build. If the BuildConfig in the generator doesn't match,
	// a build will not be generated.
	LastVersion *int64

	// Env contains additional environment variables you want to pass into a builder container.
	Env []kapi.EnvVar

	// TriggeredBy describes which triggers started the most recent update to the
	// buildconfig and contains information about those triggers.
	TriggeredBy []BuildTriggerCause

	// DockerStrategyOptions contains additional docker-strategy specific options for the build
	DockerStrategyOptions *DockerStrategyOptions

	// SourceStrategyOptions contains additional source-strategy specific options for the build
	SourceStrategyOptions *SourceStrategyOptions
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BinaryBuildRequestOptions struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	AsFile string

	// TODO: support structs in query arguments in the future (inline and nested fields)

	// Commit is the value identifying a specific commit
	Commit string

	// Message is the description of a specific commit
	Message string

	// AuthorName of the source control user
	AuthorName string

	// AuthorEmail of the source control user
	AuthorEmail string

	// CommitterName of the source control user
	CommitterName string

	// CommitterEmail of the source control user
	CommitterEmail string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildLogOptions is the REST options for a build log
type BuildLogOptions struct {
	metav1.TypeMeta

	// Container for which to return logs
	Container string
	// Follow if true indicates that the build log should be streamed until
	// the build terminates.
	Follow bool
	// If true, return previous build logs.
	Previous bool
	// A relative time in seconds before the current time from which to show logs. If this value
	// precedes the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	SinceSeconds *int64
	// An RFC3339 timestamp from which to show logs. If this value
	// precedes the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	SinceTime *metav1.Time
	// If true, add an RFC3339 or RFC3339Nano timestamp at the beginning of every line
	// of log output.
	Timestamps bool
	// If set, the number of lines from the end of the logs to show. If not specified,
	// logs are shown from the creation of the container or sinceSeconds or sinceTime
	TailLines *int64
	// If set, the number of bytes to read from the server before terminating the
	// log output. This may not display a complete final line of logging, and may return
	// slightly more or slightly less than the specified limit.
	LimitBytes *int64

	// NoWait if true causes the call to return immediately even if the build
	// is not available yet. Otherwise the server will wait until the build has started.
	NoWait bool

	// Version of the build for which to view logs.
	Version *int64
}

// SecretSpec specifies a secret to be included in a build pod and its corresponding mount point
type SecretSpec struct {
	// SecretSource is a reference to the secret
	SecretSource kapi.LocalObjectReference

	// MountPath is the path at which to mount the secret
	MountPath string
}
