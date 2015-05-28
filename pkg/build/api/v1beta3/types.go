package v1beta3

import (
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// BuildLabel is the key of a Pod label whose value is the Name of a Build which is run.
const BuildLabel = "build"

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the execution and a reference to the Pod which executed the build.
type Build struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec is all the inputs used to execute the build.
	Spec BuildSpec `json:"spec,omitempty"`

	// Status is the current status of the build.
	Status BuildStatus `json:"status,omitempty"`
}

// BuildSpec encapsulates all the inputs necessary to represent a build.
type BuildSpec struct {
	// ServiceAccount is the name of the ServiceAccount to use to run the pod
	// created by this build.
	// The pod will be allowed to use secrets referenced by the ServiceAccount
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// Source describes the SCM in use.
	Source BuildSource `json:"source,omitempty"`

	// Revision is the information from the source for a specific repo snapshot.
	// This is optional.
	Revision *SourceRevision `json:"revision,omitempty"`

	// Strategy defines how to perform a build.
	Strategy BuildStrategy `json:"strategy"`

	// Output describes the Docker image the Strategy should produce.
	Output BuildOutput `json:"output,omitempty"`

	// Compute resource requirements to execute the build
	Resources kapi.ResourceRequirements `json:"resources,omitempty" description:"the desired compute resources the build should have"`
}

// BuildStatus contains the status of a build
type BuildStatus struct {
	// Phase is the point in the build lifecycle.
	Phase BuildPhase `json:"phase"`

	// Cancelled describes if a cancelling event was triggered for the build.
	Cancelled bool `json:"cancelled,omitempty"`

	// A human readable message indicating details about why the build has this status
	Message string `json:"message,omitempty"`

	// StartTimestamp is a timestamp representing the server time when this Build started
	// running in a Pod.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *util.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp is a timestamp representing the server time when this Build was
	// finished, whether that build failed or succeeded.  It reflects the time at which
	// the Pod running the Build terminated.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTimestamp *util.Time `json:"completionTimestamp,omitempty"`

	// Duration contains time.Duration object describing build time.
	Duration time.Duration `json:"duration,omitempty"`

	// Config is an ObjectReference to the BuildConfig this Build is based on.
	Config *kapi.ObjectReference `json:"config,omitempty"`
}

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

	// SourceSecret is the name of a Secret that would be used for setting
	// up the authentication for cloning private repository.
	// The secret contains valid credentials for remote repository, where the
	// data's key represent the authentication method to be used and value is
	// the base64 encoded credentials. Supported auth methods are: ssh-privatekey.
	SourceSecret *kapi.LocalObjectReference `json:"sourceSecret,omitempty" description:"supported auth methods are: ssh-privatekey"`
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

	// SourceStrategy holds the parameters to the Source build strategy.
	SourceStrategy *SourceBuildStrategy `json:"sourceStrategy,omitempty"`

	// CustomStrategy holds the parameters to the Custom build strategy
	CustomStrategy *CustomBuildStrategy `json:"customStrategy,omitempty"`
}

// BuildStrategyType describes a particular way of performing a build.
type BuildStrategyType string

// Valid values for BuildStrategyType.
const (
	// DockerBuildStrategyType performs builds using a Dockerfile.
	DockerBuildStrategyType BuildStrategyType = "Docker"

	// SourceBuildStrategyType performs builds build using Source To Images with a Git repository
	// and a builder image.
	SourceBuildStrategyType BuildStrategyType = "Source"

	// CustomBuildStrategyType performs builds using custom builder Docker image.
	CustomBuildStrategyType BuildStrategyType = "Custom"
)

// CustomBuildStrategy defines input parameters specific to Custom build.
type CustomBuildStrategy struct {
	// From is reference to one of ImageStreamImage, ImageStreamTag or DockerImage
	// from which the docker image should be pulled.
	From *kapi.ObjectReference `json:"from,omitempty" description:"reference to one of ImageStreamImage, ImageStreamTag or DockerImage"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty" description:"supported type: dockercfg"`

	// Additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool `json:"exposeDockerSocket,omitempty"`
}

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// From is reference to one of ImageStreamImage, ImageStreamTag or DockerImage
	// from which the docker image should be pulled. The resulting image will be
	// used in the FROM line of the Dockerfile for this build.
	From *kapi.ObjectReference `json:"from,omitempty" description:"reference to one of ImageStreamImage, ImageStreamTag or DockerImage"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty" description:"supported type: dockercfg"`

	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool `json:"noCache,omitempty"`
}

// SourceBuildStrategy defines input parameters specific to an Source build.
type SourceBuildStrategy struct {
	// From is reference to one of ImageStreamImage, ImageStreamTag or DockerImage
	// from which the docker image should be pulled. The resulting image will be
	// used as the builder image for this build.
	From *kapi.ObjectReference `json:"from,omitempty" description:"reference to one of ImageStreamImage, ImageStreamTag or DockerImage"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty" description:"supported type: dockercfg"`

	// Additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// Scripts is the location of Source scripts
	Scripts string `json:"scripts,omitempty"`

	// Incremental flag forces the Source build to do incremental builds if true.
	Incremental bool `json:"incremental,omitempty"`
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// To defines an optional ImageStream to push the output of this build to. The namespace
	// may be empty, in which case the ImageStream will be looked for in the namespace of
	// the build. Kind must be 'ImageStreamTag' or 'DockerImage'.  This value will be used
	// to look up a Docker image repository to push to.
	To *kapi.ObjectReference `json:"to,omitempty" description:"reference to ImageStreamTag or DockerImage"`

	// PushSecret is the name of a Secret that would be used for setting
	// up the authentication for executing the Docker push to authentication
	// enabled Docker Registry (or Docker Hub).
	PushSecret *kapi.LocalObjectReference `json:"pushSecret,omitempty" description:"supported type: dockercfg"`
}

// BuildConfigLabel is the key of a Build label whose value is the ID of a BuildConfig
// on which the Build is based.
const BuildConfigLabel = "buildconfig"

// BuildConfig is a template which can be used to create new builds.
type BuildConfig struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds all the input necessary to produce a new build, and the conditions when
	// to trigger them.
	Spec BuildConfigSpec `json:"spec"`
	// Status holds any relevant information about a build config
	Status BuildConfigStatus `json:"status"`
}

// BuildConfigSpec describes when and how builds are created
type BuildConfigSpec struct {
	// Triggers determine how new Builds can be launched from a BuildConfig. If no triggers
	// are defined, a new build can only occur as a result of an explicit client build creation.
	Triggers []BuildTriggerPolicy `json:"triggers"`

	BuildSpec `json:",inline"`
}

// BuildConfigStatus contains current state of the build config object.
type BuildConfigStatus struct {
	// LastVersion is used to inform about number of last triggered build.
	LastVersion int `json:"lastVersion"`
}

// WebHookTrigger is a trigger that gets invoked using a webhook type of post
type WebHookTrigger struct {
	// Secret used to validate requests.
	Secret string `json:"secret,omitempty"`
}

// ImageChangeTrigger allows builds to be triggered when an ImageStream changes
type ImageChangeTrigger struct {
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

	// TriggeredByImage is the image name that triggered this build.
	TriggeredByImage string `json:"triggedByImage,omitempty"`
}

// BuildLogOptions is the REST options for a build log
type BuildLogOptions struct {
	kapi.TypeMeta

	// Follow if true indicates that the build log should be streamed until
	// the build terminates.
	Follow bool `json:"follow,omitempty" description:"if true indicates that the log should be streamed; defaults to false"`

	// NoWait if true causes the call to return immediately even if the build
	// is not available yet. Otherwise the server will wait until the build has started.
	NoWait bool `json:"nowait,omitempty" description:"if true indicates that the server should not wait for a log to be available before returning; defaults to false"`
}
