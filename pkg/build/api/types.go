package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the execution and a reference to the Pod which executed the build.
type Build struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Parameters are all the inputs used to create the build pod.
	Parameters BuildParameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// Status is the current status of the build.
	Status BuildStatus `json:"status,omitempty" yaml:"status,omitempty"`

	// PodName is the name of the pod that is used to execute the build
	PodName string `json:"podName,omitempty" yaml:"podName,omitempty"`

	// Cancelled describes if a cancelling event was triggered for the build.
	Cancelled bool `json:"cancelled,omitempty" yaml:"cancelled,omitempty"`
}

// BuildParameters encapsulates all the inputs necessary to represent a build.
type BuildParameters struct {
	// Source describes the SCM in use.
	Source BuildSource `json:"source,omitempty" yaml:"source,omitempty"`

	// Revision is the information from the source for a specific repo snapshot.
	// This is optional.
	Revision *SourceRevision `json:"revision,omitempty" yaml:"revision,omitempty"`

	// Strategy defines how to perform a build.
	Strategy BuildStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`

	// Output describes the Docker image the Strategy should produce.
	Output BuildOutput `json:"output,omitempty" yaml:"output,omitempty"`
}

// BuildStatus represents the status of a build at a point in time.
type BuildStatus string

// Valid values for BuildStatus.
const (
	// BuildNew is automatically assigned to a newly created build.
	BuildStatusNew BuildStatus = "New"

	// BuildPending indicates that a pod name has been assigned and a build is
	// about to start running.
	BuildStatusPending BuildStatus = "Pending"

	// BuildRunning indicates that a pod has been created and a build is running.
	BuildStatusRunning BuildStatus = "Running"

	// BuildComplete indicates that a build has been successful.
	BuildStatusComplete BuildStatus = "Complete"

	// BuildFailed indicates that a build has executed and failed.
	BuildStatusFailed BuildStatus = "Failed"

	// BuildError indicates that an error prevented the build from executing.
	BuildStatusError BuildStatus = "Error"

	// BuildStatusCancelled indicates that a running/pending build was stopped from executing.
	BuildStatusCancelled BuildStatus = "Cancelled"
)

// BuildSourceType is the type of SCM used
type BuildSourceType string

// Valid values for BuildSourceType.
const (
	//BuildGitSource is a Git SCM
	BuildSourceGit BuildSourceType = "Git"
)

// BuildSource is the SCM used for the build
type BuildSource struct {
	Type BuildSourceType `json:"type,omitempty" yaml:"type,omitempty"`
	Git  *GitBuildSource `json:"git,omitempty" yaml:"git,omitempty"`
}

// SourceRevision is the revision or commit information from the source for the build
type SourceRevision struct {
	Type BuildSourceType    `json:"type,omitempty" yaml:"type,omitempty"`
	Git  *GitSourceRevision `json:"git,omitempty" yaml:"git,omitempty"`
}

// GitSourceRevision is the commit information from a git source for a build
type GitSourceRevision struct {
	// Commit is the commit hash identifying a specific commit
	Commit string `json:"commit,omitempty" yaml:"commit,omitempty"`

	// Author is the author of a specific commit
	Author SourceControlUser `json:"author,omitempty" yaml:"author,omitempty"`

	// Committer is the commiter of a specific commit
	Committer SourceControlUser `json:"committer,omitempty" yaml:"committer,omitempty"`

	// Message is the description of a specific commit
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// GitBuildSource defines the parameters of a Git SCM
type GitBuildSource struct {
	// URI points to the source that will be built. The structure of the source
	// will depend on the type of build to run
	URI string `json:"uri,omitempty" yaml:"uri,omitempty"`

	// Ref is the branch/tag/ref to build.
	Ref string `json:"ref,omitempty" yaml:"ref,omitempty"`
}

// SourceControlUser defines the identity of a user of source control
type SourceControlUser struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

// BuildStrategy contains the details of how to perform a build.
type BuildStrategy struct {
	// Type is the kind of build strategy.
	Type BuildStrategyType `json:"type,omitempty" yaml:"type,omitempty"`

	// DockerStrategy holds the parameters to the Docker build strategy.
	DockerStrategy *DockerBuildStrategy `json:"dockerStrategy,omitempty" yaml:"dockerStrategy,omitempty"`

	// STIStrategy holds the parameters to the STI build strategy.
	STIStrategy *STIBuildStrategy `json:"stiStrategy,omitempty" yaml:"stiStrategy,omitempty"`

	// CustomStrategy holds the parameters to the Custom build strategy.
	CustomStrategy *CustomBuildStrategy `json:"customStrategy,omitempty" yaml:"customStrategy,omitempty"`
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

// CustomBuildStrategy defines input parameters specific to Custom build.
type CustomBuildStrategy struct {
	// Image is the image required to execute the build. If not specified
	// a validation error is returned.
	Image string `json:"image" yaml:"image"`

	// Additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool `json:"exposeDockerSocket,omitempty" yaml:"exposeDockerSocket,omitempty"`
}

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// ContextDir is used as the Docker build context. It is a path for a directory within the
	// application source directory structure (as referenced in the BuildSource. See GitBuildSource
	// for an example.)
	ContextDir string `json:"contextDir,omitempty" yaml:"contextDir,omitempty"`

	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool `json:"noCache,omitempty" yaml:"noCache,omitempty"`
}

// STIBuildStrategy defines input parameters specific to an STI build.
type STIBuildStrategy struct {
	// Image is the image used to execute the build.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// Scripts is the location of STI scripts
	Scripts string `json:"scripts,omitempty" yaml:"scripts,omitempty"`

	// Clean flag forces the STI build to not do incremental builds if true.
	Clean bool `json:"clean,omitempty" yaml:"clean,omitempty"`
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// ImageTag is the tag to give to the image resulting from the build.
	ImageTag string `json:"imageTag,omitempty" yaml:"imageTag,omitempty"`

	// Registry is the Docker registry which should receive the resulting built image via push.
	Registry string `json:"registry,omitempty" yaml:"registry,omitempty"`
}

// BuildConfigLabel is the key of a Build label whose value is the ID of a BuildConfig
// on which the Build is based.
const BuildConfigLabel = "buildconfig"

// BuildConfig is a template which can be used to create new builds.
type BuildConfig struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Triggers determine how new Builds can be launched from a BuildConfig. If no triggers
	// are defined, a new build can only occur as a result of an explicit client build creation.
	Triggers []BuildTriggerPolicy `json:"triggers,omitempty" yaml:"triggers,omitempty"`

	// Parameters holds all the input necessary to produce a new build.
	Parameters BuildParameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// WebHookTrigger is a trigger that gets invoked using a webhook type of post
type WebHookTrigger struct {
	// Secret used to validate requests.
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// BuildTriggerPolicy describes a policy for a single trigger that results in a new Build.
type BuildTriggerPolicy struct {
	// Type is the type of build trigger
	Type BuildTriggerType `json:"type,omitempty" yaml:"type,omitempty"`

	// GithubWebHook contains the parameters for a Github webhook type of trigger
	GithubWebHook *WebHookTrigger `json:"github,omitempty" yaml:"github,omitempty"`

	// GenericWebHook contains the parameters for a Generic webhook type of trigger
	GenericWebHook *WebHookTrigger `json:"generic,omitempty" yaml:"generic,omitempty"`
}

// BuildTriggerType refers to a specific BuildTriggerPolicy implementation.
type BuildTriggerType string

const (
	// GithubWebHookType represents a trigger that launches builds on
	// Github webhook invocations
	GithubWebHookType BuildTriggerType = "github"

	// GenericWebHookType represents a trigger that launches builds on
	// generic webhook invocations
	GenericWebHookType BuildTriggerType = "generic"
)

// BuildList is a collection of Builds.
type BuildList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items         []Build `json:"items" yaml:"items"`
}

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items         []BuildConfig `json:"items" yaml:"items"`
}

// GenericWebHookEvent is the payload expected for a generic webhook post
type GenericWebHookEvent struct {
	// Type is the type of source repository
	Type BuildSourceType `json:"type,omitempty" yaml:"type,omitempty"`

	// Git is the git information if the Type is BuildSourceGit
	Git *GitInfo `json:"git,omitempty" yaml:"git,omitempty"`
}

// GitInfo is the aggregated git information for a generic webhook post
type GitInfo struct {
	GitBuildSource    `json:",inline" yaml:",inline"`
	GitSourceRevision `json:",inline" yaml:",inline"`
}
