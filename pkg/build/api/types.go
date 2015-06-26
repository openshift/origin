package api

import (
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

const (
	// BuildAnnotation is an annotation that identifies a Pod as being for a Build
	BuildAnnotation = "openshift.io/build.name"
	// BuildLabel is the key of a Pod label whose value is the Name of a Build which is run.
	BuildLabel = "build"
)

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the execution and a reference to the Pod which executed the build.
type Build struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Parameters are all the inputs used to create the build pod.
	Parameters BuildParameters

	// Status is the current status of the build.
	Status BuildStatus

	// Message is a human readable message indicating details about why the build has this status
	Message string

	// Cancelled describes if a cancelling event was triggered for the build.
	Cancelled bool

	// StartTimestamp is a timestamp representing the server time when this Build started
	// running in a Pod.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *util.Time

	// CompletionTimestamp is a timestamp representing the server time when this Build was
	// finished, whether that build failed or succeeded.  It reflects the time at which
	// the Pod running the Build terminated.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTimestamp *util.Time

	// Duration contains time.Duration object describing build time.
	Duration time.Duration

	// Config is an ObjectReference to the BuildConfig this Build is based on.
	Config *kapi.ObjectReference
}

// BuildParameters encapsulates all the inputs necessary to represent a build.
type BuildParameters struct {
	// ServiceAccount is the name of the ServiceAccount to use to run the pod
	// created by this build.
	// The pod will be allowed to use secrets referenced by the ServiceAccount
	ServiceAccount string

	// Source describes the SCM in use.
	Source BuildSource

	// Revision is the information from the source for a specific repo snapshot.
	// This is optional.
	Revision *SourceRevision

	// Strategy defines how to perform a build.
	Strategy BuildStrategy

	// Output describes the Docker image the Strategy should produce.
	Output BuildOutput

	// Compute resource requirements to execute the build
	Resources kapi.ResourceRequirements
}

// BuildStatus represents the status of a build at a point in time.
type BuildStatus string

// Valid values for BuildStatus.
const (
	// BuildStatusNew is automatically assigned to a newly created build.
	BuildStatusNew BuildStatus = "New"

	// BuildStatusPending indicates that a pod name has been assigned and a build is
	// about to start running.
	BuildStatusPending BuildStatus = "Pending"

	// BuildStatusRunning indicates that a pod has been created and a build is running.
	BuildStatusRunning BuildStatus = "Running"

	// BuildStatusComplete indicates that a build has been successful.
	BuildStatusComplete BuildStatus = "Complete"

	// BuildStatusFailed indicates that a build has executed and failed.
	BuildStatusFailed BuildStatus = "Failed"

	// BuildStatusError indicates that an error prevented the build from executing.
	BuildStatusError BuildStatus = "Error"

	// BuildStatusCancelled indicates that a running/pending build was stopped from executing.
	BuildStatusCancelled BuildStatus = "Cancelled"
)

// BuildSourceType is the type of SCM used
type BuildSourceType string

// Valid values for BuildSourceType.
const (
	//BuildSourceGit is a Git SCM
	BuildSourceGit BuildSourceType = "Git"
)

// BuildSource is the SCM used for the build
type BuildSource struct {
	// Type of source control management system
	Type BuildSourceType

	// Git contains optional information about git build source
	Git *GitBuildSource

	// ContextDir specifies the sub-directory where the source code for the application exists.
	// This allows to have buildable sources in directory other than root of
	// repository.
	ContextDir string

	// SourceSecret is the name of a Secret that would be used for setting
	// up the authentication for cloning private repository.
	// The secret contains valid credentials for remote repository, where the
	// data's key represent the authentication method to be used and value is
	// the base64 encoded credentials. Supported auth methods are: ssh-privatekey.
	SourceSecret *kapi.LocalObjectReference
}

// SourceRevision is the revision or commit information from the source for the build
type SourceRevision struct {
	// Type of the build source
	Type BuildSourceType

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

// GitBuildSource defines the parameters of a Git SCM
type GitBuildSource struct {
	// URI points to the source that will be built. The structure of the source
	// will depend on the type of build to run
	URI string

	// Ref is the branch/tag/ref to build.
	Ref string
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
	// Type is the kind of build strategy.
	Type BuildStrategyType

	// DockerStrategy holds the parameters to the Docker build strategy.
	DockerStrategy *DockerBuildStrategy

	// SourceStrategy holds the parameters to the STI build strategy.
	SourceStrategy *SourceBuildStrategy

	// CustomStrategy holds the parameters to the Custom build strategy.
	CustomStrategy *CustomBuildStrategy
}

// BuildStrategyType describes a particular way of performing a build.
type BuildStrategyType string

// Valid values for BuildStrategyType.
const (
	// DockerBuildStrategyType performs builds using a Dockerfile.
	DockerBuildStrategyType BuildStrategyType = "Docker"

	// SourceBuildStrategyType performs builds build using Source To Images with a Git repository
	// and a builder image.
	SourceBuildStrategyType BuildStrategyType = "STI"

	// CustomBuildStrategyType performs builds using the custom builder Docker image.
	CustomBuildStrategyType BuildStrategyType = "Custom"
)

const (
	// CustomBuildStrategyBaseImageKey is the environment variable that indicates the base image to be used when
	// performing a custom build, if needed.
	CustomBuildStrategyBaseImageKey = "OPENSHIFT_CUSTOM_BUILD_BASE_IMAGE"
)

// CustomBuildStrategy defines input parameters specific to Custom build.
type CustomBuildStrategy struct {
	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool

	// From is reference to an ImageStream, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference
}

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool

	// From is reference to an ImageStream, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	// the resulting image will be used in the FROM line of the Dockerfile for this build.
	From *kapi.ObjectReference

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference
}

// SourceBuildStrategy defines input parameters specific to an STI build.
type SourceBuildStrategy struct {
	// From is reference to an ImageStream, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar

	// Scripts is the location of STI scripts
	Scripts string

	// Incremental flag forces the STI build to do incremental builds if true.
	Incremental bool
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// To defines an optional ImageStream to push the output of this build to. The namespace
	// may be empty, in which case the named ImageStream will be retrieved from the namespace
	// of the build. Kind must be set to 'ImageStream' and is the only supported value. If set,
	// this field takes priority over DockerImageReference. This value will be used to look up
	// a Docker image repository to push to. Failure to find the To will result in a build error.
	To *kapi.ObjectReference

	// PushSecret is the name of a Secret that would be used for setting
	// up the authentication for executing the Docker push to authentication
	// enabled Docker Registry (or Docker Hub).
	PushSecret *kapi.LocalObjectReference

	// Tag is the "version name" that will be associated with the output image. This
	// field is only used if the To field is set, and is ignored when DockerImageReference is used.
	// This value represents a consistent name for a set of related changes (v1, 5.x, 5.5, dev, stable)
	// and defaults to the preferred tag for "To" if not specified.
	Tag string

	// DockerImageReference is the full name of an image ([registry/]name[:tag]), and will be the
	// value sent to Docker push at the end of a build if the To field is not defined.
	DockerImageReference string
}

// BuildConfigLabel is the key of a Build label whose value is the ID of a BuildConfig
// on which the Build is based.
const BuildConfigLabel = "buildconfig"

// BuildConfig is a template which can be used to create new builds.
type BuildConfig struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Triggers determine how new Builds can be launched from a BuildConfig. If no triggers
	// are defined, a new build can only occur as a result of an explicit client build creation.
	Triggers []BuildTriggerPolicy

	// LastVersion is used to inform about number of last triggered build.
	LastVersion int

	// Parameters holds all the input necessary to produce a new build. A build config may only
	// define either the Output.To or Output.DockerImageReference fields, but not both.
	Parameters BuildParameters
}

// WebHookTrigger is a trigger that gets invoked using a webhook type of post
type WebHookTrigger struct {
	// Secret used to validate requests.
	Secret string
}

// ImageChangeTrigger allows builds to be triggered when an ImageStream changes
type ImageChangeTrigger struct {
	// LastTriggeredImageID is used internally by the ImageChangeController to save last
	// used image ID for build
	LastTriggeredImageID string
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
}

// BuildTriggerType refers to a specific BuildTriggerPolicy implementation.
type BuildTriggerType string

const (
	// GitHubWebHookBuildTriggerType represents a trigger that launches builds on
	// GitHub webhook invocations
	GitHubWebHookBuildTriggerType BuildTriggerType = "GitHub"

	// GenericWebHookBuildTriggerType represents a trigger that launches builds on
	// generic webhook invocations
	GenericWebHookBuildTriggerType BuildTriggerType = "Generic"

	// ImageChangeBuildTriggerType represents a trigger that launches builds on
	// availability of a new version of an image
	ImageChangeBuildTriggerType BuildTriggerType = "ImageChange"
)

// BuildList is a collection of Builds.
type BuildList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of builds
	Items []Build
}

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of build configs
	Items []BuildConfig
}

// GenericWebHookEvent is the payload expected for a generic webhook post
type GenericWebHookEvent struct {
	// Type is the type of source repository
	Type BuildSourceType

	// Git is the git information if the Type is BuildSourceGit
	Git *GitInfo
}

// GitInfo is the aggregated git information for a generic webhook post
type GitInfo struct {
	GitBuildSource
	GitSourceRevision

	// Refs is a list of GitRefs for the provided repo - generally sent
	// when used from a post-receive hook. This field is optional and is
	// used when sending multiple refs
	Refs []GitRefInfo
}

// GitRefInfo is a single ref
type GitRefInfo struct {
	GitBuildSource
	GitSourceRevision
}

// BuildLog is the (unused) resource associated with the build log redirector
type BuildLog struct {
	kapi.TypeMeta
	kapi.ListMeta
}

// BuildRequest is the resource used to pass parameters to build generator
type BuildRequest struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Revision is the information from the source for a specific repo snapshot.
	Revision *SourceRevision

	// TriggeredByImage is the Image that triggered this build.
	TriggeredByImage *kapi.ObjectReference
}

// BuildLogOptions is the REST options for a build log
type BuildLogOptions struct {
	kapi.TypeMeta

	// Follow if true indicates that the build log should be streamed until
	// the build terminates.
	Follow bool

	// NoWait if true causes the call to return immediately even if the build
	// is not available yet. Otherwise the server will wait until the build has started.
	NoWait bool
}
