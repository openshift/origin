package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// BuildLabel is the key of a Pod label whose value is the Name of a Build which is run.
const BuildLabel = "build"

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the execution and a reference to the Pod which executed the build.
type Build struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Parameters are all the inputs used to create the build pod.
	Parameters BuildParameters `json:"parameters,omitempty"`

	// Status is the current status of the build.
	Status BuildStatus `json:"status,omitempty"`

	// A human readable message indicating details about why the build has this status
	Message string `json:"message,omitempty"`

	// Cancelled describes if a cancelling event was triggered for the build.
	Cancelled bool `json:"cancelled,omitempty"`

	// StartTimestamp is a timestamp representing the server time when this Build started
	// running in a Pod.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *util.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp is a timestamp representing the server time when this Build was
	// finished, whether that build failed or succeeded.  It reflects the time at which
	// the Pod running the Build terminated.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTimestamp *util.Time `json:"completionTimestamp,omitempty"`

	// Config is an ObjectReference to the BuildConfig this Build is based on.
	Config *kapi.ObjectReference `json:"config,omitempty"`
}

// BuildParameters encapsulates all the inputs necessary to represent a build.
type BuildParameters struct {
	// Source describes the SCM in use.
	Source BuildSource `json:"source,omitempty"`

	// Revision is the information from the source for a specific repo snapshot.
	// This is optional.
	Revision *SourceRevision `json:"revision,omitempty"`

	// Strategy defines how to perform a build.
	Strategy BuildStrategy `json:"strategy"`

	// Output describes the Docker image the Strategy should produce.
	Output BuildOutput `json:"output,omitempty"`
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
	Type BuildSourceType `json:"type,omitempty"`
	Git  *GitBuildSource `json:"git,omitempty"`

	// Specify the sub-directory where the source code for the application exists.
	// This allows to have buildable sources in directory other than root of
	// repository.
	ContextDir string `json:"contextDir,omitempty"`
}

// SourceRevision is the revision or commit information from the source for the build
type SourceRevision struct {
	Type BuildSourceType    `json:"type,omitempty"`
	Git  *GitSourceRevision `json:"git,omitempty"`
}

// GitSourceRevision is the commit information from a git source for a build
type GitSourceRevision struct {
	// Commit is the commit hash identifying a specific commit
	Commit string `json:"commit,omitempty"`

	// Author is the author of a specific commit
	Author SourceControlUser `json:"author,omitempty"`

	// Committer is the committer of a specific commit
	Committer SourceControlUser `json:"committer,omitempty"`

	// Message is the description of a specific commit
	Message string `json:"message,omitempty"`
}

// GitBuildSource defines the parameters of a Git SCM
type GitBuildSource struct {
	// URI points to the source that will be built. The structure of the source
	// will depend on the type of build to run
	URI string `json:"uri,omitempty"`

	// Ref is the branch/tag/ref to build.
	Ref string `json:"ref,omitempty"`
}

// SourceControlUser defines the identity of a user of source control
type SourceControlUser struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// BuildStrategy contains the details of how to perform a build.
type BuildStrategy struct {
	// Type is the kind of build strategy.
	Type BuildStrategyType `json:"type"`

	// DockerStrategy holds the parameters to the Docker build strategy.
	DockerStrategy *DockerBuildStrategy `json:"dockerStrategy,omitempty"`

	// STIStrategy holds the parameters to the STI build strategy.
	STIStrategy *STIBuildStrategy `json:"stiStrategy,omitempty"`

	// CustomStrategy holds the parameters to the Custom build strategy.
	CustomStrategy *CustomBuildStrategy `json:"customStrategy,omitempty"`
}

// BuildStrategyType describes a particular way of performing a build.
type BuildStrategyType string

// Valid values for BuildStrategyType.
const (
	// DockerBuildStrategyType performs builds using a Dockerfile.
	DockerBuildStrategyType BuildStrategyType = "Docker"

	// STIBuildStrategyType performs builds build using Source To Images with a Git repository
	// and a builder image.
	STIBuildStrategyType BuildStrategyType = "STI"

	// CustomBuildStrategyType performs builds using the custom builder Docker image.
	CustomBuildStrategyType BuildStrategyType = "Custom"
)

const (
	// CustomBuildStrategyBaseImageKey is the environment variable that indicates the base image to be used when
	// performing a custom build, if needed.
	CustomBuildStrategyBaseImageKey = "OPENSHIFT_CUSTOM_BUILD_BASE_IMAGE"

	// DefaultImageTag is used when an image tag is needed and the configuration does not specify a tag to use.
	DefaultImageTag string = "latest"
)

// CustomBuildStrategy defines input parameters specific to Custom build.
type CustomBuildStrategy struct {
	// Image is the image required to execute the build. If not specified
	// a validation error is returned.
	Image string `json:"image"`

	// Additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool `json:"exposeDockerSocket,omitempty"`
}

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool `json:"noCache,omitempty"`

	// Image is optional and indicates the image that the dockerfile for this
	// build should "FROM".  If present, the build process will substitute this value
	// into the FROM line of the dockerfile.
	Image string `json:"image,omitempty"`
}

// STIBuildStrategy defines input parameters specific to an STI build.
type STIBuildStrategy struct {
	// Image is the image used to execute the build.
	// Only valid if From is not present.
	Image string `json:"image,omitempty"`

	// From is reference to an image stream from where the docker image should be pulled
	From *kapi.ObjectReference `json:"from,omitempty"`

	// Tag is the name of image stream tag to be used as the build image, it only
	// applies when From is specified.
	Tag string `json:"tag,omitempty`

	// Additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// Scripts is the location of STI scripts
	Scripts string `json:"scripts,omitempty"`

	// Incremental flag forces the STI build to do incremental builds if true.
	Incremental bool `json:"incremental,omitempty"`
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// To defines an optional ImageStream to push the output of this build to. The namespace
	// may be empty, in which case the named ImageStream will be retrieved from the namespace
	// of the build. Kind must be set to 'ImageStream' and is the only supported value. If set,
	// this field takes priority over DockerImageReference. This value will be used to look up
	// a Docker image repository to push to. Failure to find the To will result in a build error.
	To *kapi.ObjectReference `json:"to,omitempty"`

	// pushSecretName is the name of a Secret that would be used for setting
	// up the authentication for executing the Docker push to authentication
	// enabled Docker Registry (or Docker Hub).
	PushSecretName string `json:"pushSecretName,omitempty"`

	// Tag is the "version name" that will be associated with the output image. This
	// field is only used if the To field is set, and is ignored when DockerImageReference is used.
	// This value represents a consistent name for a set of related changes (v1, 5.x, 5.5, dev, stable)
	// and defaults to the preferred tag for "To" if not specified.
	Tag string `json:"tag,omitempty"`

	// DockerImageReference is the full name of an image ([registry/]name[:tag]), and will be the
	// value sent to Docker push at the end of a build if the To field is not defined.
	DockerImageReference string `json:"dockerImageReference,omitempty"`
}

// BuildConfigLabel is the key of a Build label whose value is the ID of a BuildConfig
// on which the Build is based.
const BuildConfigLabel = "buildconfig"

// BuildConfig is a template which can be used to create new builds.
type BuildConfig struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Triggers determine how new Builds can be launched from a BuildConfig. If no triggers
	// are defined, a new build can only occur as a result of an explicit client build creation.
	Triggers []BuildTriggerPolicy `json:"triggers,omitempty"`

	// LastVersion is used to inform about number of last triggered build.
	LastVersion int `json:"lastVersion,omitempty"`

	// Parameters holds all the input necessary to produce a new build. A build config may only
	// define either the Output.To or Output.DockerImageReference fields, but not both.
	Parameters BuildParameters `json:"parameters,omitempty"`
}

// WebHookTrigger is a trigger that gets invoked using a webhook type of post
type WebHookTrigger struct {
	// Secret used to validate requests.
	Secret string `json:"secret,omitempty"`
}

// ImageChangeTrigger allows builds to be triggered when an ImageStream changes
type ImageChangeTrigger struct {
	// Image is used to specify the value in the BuildConfig to replace with the
	// immutable image id supplied by the ImageStream when this trigger fires.
	Image string `json:"image"`
	// From is a reference to an image stream to watch for changes. This field takes
	// precedence over ImageRepositoryRef, which is deprecated and will be removed in v1beta2. The
	// Kind may be left blank, in which case it defaults to "ImageStream". The "Name" is
	// the only required subfield - if Namespace is blank, the namespace of the current build
	// trigger will be used.
	From kapi.ObjectReference `json:"from"`
	// Tag is the name of an image stream tag to watch for changes.
	Tag string `json:"tag,omitempty"`
	// LastTriggeredImageID is used internally by the ImageChangeController to save last
	// used image ID for build
	LastTriggeredImageID string `json:"lastTriggeredImageID,omitempty"`
}

// BuildTriggerPolicy describes a policy for a single trigger that results in a new Build.
type BuildTriggerPolicy struct {
	// Type is the type of build trigger
	Type BuildTriggerType `json:"type,omitempty"`

	// GithubWebHook contains the parameters for a GitHub webhook type of trigger
	GithubWebHook *WebHookTrigger `json:"github,omitempty"`

	// GenericWebHook contains the parameters for a Generic webhook type of trigger
	GenericWebHook *WebHookTrigger `json:"generic,omitempty"`

	// ImageChange contains parameters for an ImageChange type of trigger
	ImageChange *ImageChangeTrigger `json:"imageChange,omitempty"`
}

// BuildTriggerType refers to a specific BuildTriggerPolicy implementation.
type BuildTriggerType string

const (
	// GithubWebHookBuildTriggerType represents a trigger that launches builds on
	// GitHub webhook invocations
	GithubWebHookBuildTriggerType BuildTriggerType = "github"

	// GenericWebHookBuildTriggerType represents a trigger that launches builds on
	// generic webhook invocations
	GenericWebHookBuildTriggerType BuildTriggerType = "generic"

	// ImageChangeBuildTriggerType represents a trigger that launches builds on
	// availability of a new version of an image
	ImageChangeBuildTriggerType BuildTriggerType = "imageChange"
)

// BuildList is a collection of Builds.
type BuildList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Build `json:"items"`
}

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []BuildConfig `json:"items"`
}

// GenericWebHookEvent is the payload expected for a generic webhook post
type GenericWebHookEvent struct {
	// Type is the type of source repository
	Type BuildSourceType `json:"type,omitempty"`

	// Git is the git information if the Type is BuildSourceGit
	Git *GitInfo `json:"git,omitempty"`
}

// GitInfo is the aggregated git information for a generic webhook post
type GitInfo struct {
	GitBuildSource    `json:",inline"`
	GitSourceRevision `json:",inline"`
}

// BuildLog is the (unused) resource associated with the build log redirector
type BuildLog struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
}

// BuildRequest is the resource used to pass parameters to build generator
type BuildRequest struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Revision is the information from the source for a specific repo snapshot.
	Revision *SourceRevision `json:"revision,omitempty"`
}
