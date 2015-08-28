package v1

import (
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the execution and a reference to the Pod which executed the build.
type Build struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Spec is all the inputs used to execute the build.
	Spec BuildSpec `json:"spec,omitempty" description:"specification of the desired behavior for a build"`

	// Status is the current status of the build.
	Status BuildStatus `json:"status,omitempty" description:"most recently observed status of a build as populated by the system"`
}

// BuildSpec encapsulates all the inputs necessary to represent a build.
type BuildSpec struct {
	// ServiceAccount is the name of the ServiceAccount to use to run the pod
	// created by this build.
	// The pod will be allowed to use secrets referenced by the ServiceAccount
	ServiceAccount string `json:"serviceAccount,omitempty" description:"the name of the service account to use to run pods created by the build, pod will be allowed to use secrets referenced by the service account"`

	// Source describes the SCM in use.
	Source BuildSource `json:"source,omitempty" description:"describes the source control management system in use"`

	// Revision is the information from the source for a specific repo snapshot.
	// This is optional.
	Revision *SourceRevision `json:"revision,omitempty" description:"specific revision in the source repository"`

	// Strategy defines how to perform a build.
	Strategy BuildStrategy `json:"strategy" description:"defines how to perform a build"`

	// Output describes the Docker image the Strategy should produce.
	Output BuildOutput `json:"output,omitempty" description:"describes the output of a build that a strategy should produce"`

	// Compute resource requirements to execute the build
	Resources kapi.ResourceRequirements `json:"resources,omitempty" description:"the desired compute resources the build should have"`

	// Optional duration in seconds, counted from the time when a build pod gets
	// scheduled in the system, that the build may be active on a node before the
	// system actively tries to terminate the build; value must be positive integer
	CompletionDeadlineSeconds *int64 `json:"completionDeadlineSeconds,omitempty" description:"optional duration in seconds the build may be active on a node before the system will actively try to mark it failed and kill associated containers; value must be a positive integer"`
}

// BuildStatus contains the status of a build
type BuildStatus struct {
	// Phase is the point in the build lifecycle.
	Phase BuildPhase `json:"phase" description:"observed point in the build lifecycle"`

	// Cancelled describes if a cancelling event was triggered for the build.
	Cancelled bool `json:"cancelled,omitempty" description:"describes if a canceling event was triggered for the build"`

	// Message is a human-readable message indicating details about why the build has this status
	Message string `json:"message,omitempty" description:"human-readable message indicating details about why the build has this status"`

	// StartTimestamp is a timestamp representing the server time when this Build started
	// running in a Pod.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *unversioned.Time `json:"startTimestamp,omitempty" description:"server time when this build started running in a pod"`

	// CompletionTimestamp is a timestamp representing the server time when this Build was
	// finished, whether that build failed or succeeded.  It reflects the time at which
	// the Pod running the Build terminated.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTimestamp *unversioned.Time `json:"completionTimestamp,omitempty" description:"server time when the pod running this build stopped running"`

	// Duration contains time.Duration object describing build time.
	Duration time.Duration `json:"duration,omitempty" description:"amount of time the build has been running"`

	// OutputDockerImageReference contains a reference to the Docker image that
	// will be built by this build. It's value is computed from
	// Build.Spec.Output.To, and should include the registry address, so that
	// it can be used to push and pull the image.
	OutputDockerImageReference string `json:"outputDockerImageReference,omitempty" description:"reference to the Docker image built by this build, computed from build.spec.output.to, and can be used to push and pull the image"`

	// Config is an ObjectReference to the BuildConfig this Build is based on.
	Config *kapi.ObjectReference `json:"config,omitempty" description:"reference to build config from which this build was derived"`
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

// BuildSourceType is the type of SCM used.
type BuildSourceType string

// Valid values for BuildSourceType.
const (
	// BuildSourceGit is a Git SCM.
	BuildSourceGit BuildSourceType = "Git"
	// BuildSourceDockerfile is an embedded Dockerfile.
	BuildSourceDockerfile BuildSourceType = "Dockerfile"
)

// BuildSource is the SCM used for the build.
type BuildSource struct {
	// Type of build input system.
	Type BuildSourceType `json:"type" description:"type of source control management system"`

	// Dockerfile is the raw contents of a Dockerfile which should be built. When this option is
	// specified, the FROM may be modified based on your strategy base image and additional ENV
	// stanzas from your strategy environment will be added after the FROM, but before the rest
	// of your Dockerfile stanzas. The Dockerfile source type may be used with other options like
	// git - in those cases the Git repo will have any innate Dockerfile replaced in the context
	// dir.
	Dockerfile *string `json:"dockerfile,omitempty" description:"the contents of a Dockerfile to build; FROM may be overridden by your strategy source, and additional ENV from your strategy will be placed before the rest of the Dockerfile stanzas"`

	// Git contains optional information about git build source
	Git *GitBuildSource `json:"git,omitempty" description:"optional information about git build source"`

	// ContextDir specifies the sub-directory where the source code for the application exists.
	// This allows to have buildable sources in directory other than root of
	// repository.
	ContextDir string `json:"contextDir,omitempty" description:"specifies sub-directory where the source code for the application exists, allows for sources to be built from a directory other than the root of a repository"`

	// SourceSecret is the name of a Secret that would be used for setting
	// up the authentication for cloning private repository.
	// The secret contains valid credentials for remote repository, where the
	// data's key represent the authentication method to be used and value is
	// the base64 encoded credentials. Supported auth methods are: ssh-privatekey.
	SourceSecret *kapi.LocalObjectReference `json:"sourceSecret,omitempty" description:"supported auth methods are: ssh-privatekey"`
}

// SourceRevision is the revision or commit information from the source for the build
type SourceRevision struct {
	// Type of the build source
	Type BuildSourceType `json:"type" description:"type of the build source"`

	// Git contains information about git-based build source
	Git *GitSourceRevision `json:"git,omitempty" description:"information about git-based build source"`
}

// GitSourceRevision is the commit information from a git source for a build
type GitSourceRevision struct {
	// Commit is the commit hash identifying a specific commit
	Commit string `json:"commit,omitempty" description:"hash identifying a specific commit"`

	// Author is the author of a specific commit
	Author SourceControlUser `json:"author,omitempty" description:"author of a specific commit"`

	// Committer is the committer of a specific commit
	Committer SourceControlUser `json:"committer,omitempty" description:"committer of a specific commit"`

	// Message is the description of a specific commit
	Message string `json:"message,omitempty" description:"description of a specific commit"`
}

// GitBuildSource defines the parameters of a Git SCM
type GitBuildSource struct {
	// URI points to the source that will be built. The structure of the source
	// will depend on the type of build to run
	URI string `json:"uri" description:"points to the source that will be built, structure of the source will depend on the type of build to run"`

	// Ref is the branch/tag/ref to build.
	Ref string `json:"ref,omitempty" description:"identifies the branch/tag/ref to build"`

	// HTTPProxy is a proxy used to reach the git repository over http
	HTTPProxy string `json:"httpProxy,omitempty" description:"specifies a http proxy to be used during git clone operations"`

	// HTTPSProxy is a proxy used to reach the git repository over https
	HTTPSProxy string `json:"httpsProxy,omitempty" description:"specifies a https proxy to be used during git clone operations"`
}

// SourceControlUser defines the identity of a user of source control
type SourceControlUser struct {
	// Name of the source control user
	Name string `json:"name,omitempty" description:"name of the source control user"`

	// Email of the source control user
	Email string `json:"email,omitempty" description:"e-mail of the source control user"`
}

// BuildStrategy contains the details of how to perform a build.
type BuildStrategy struct {
	// Type is the kind of build strategy.
	Type BuildStrategyType `json:"type" description:"identifies the type of build strategy"`

	// DockerStrategy holds the parameters to the Docker build strategy.
	DockerStrategy *DockerBuildStrategy `json:"dockerStrategy,omitempty" description:"holds parameters for the Docker build strategy"`

	// SourceStrategy holds the parameters to the Source build strategy.
	SourceStrategy *SourceBuildStrategy `json:"sourceStrategy,omitempty" description:"holds parameters to the Source build strategy"`

	// CustomStrategy holds the parameters to the Custom build strategy
	CustomStrategy *CustomBuildStrategy `json:"customStrategy,omitempty" description:"holds parameters to the Custom build strategy"`
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
	// From is reference to an DockerImage, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference `json:"from" description:"reference to an image stream, image stream tag, or image stream image from which the Docker image should be pulled"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty" description:"supported type: dockercfg"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty" description:"additional environment variables you want to pass into a builder container"`

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool `json:"exposeDockerSocket,omitempty" description:"allow running Docker commands (and build Docker images) from inside the container"`

	// ForcePull describes if the controller should configure the build pod to always pull the images
	// for the builder or only pull if it is not present locally
	ForcePull bool `json:"forcePull,omitempty" description:"forces pulling of builder image from remote registry if true"`

	// Secrets is a list of additional secrets that will be included in the build pod
	Secrets []SecretSpec `json:"secrets,omitempty" description:"a list of secrets to include in the build pod in addition to pull, push and source secrets"`
}

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// From is reference to an DockerImage, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	// the resulting image will be used in the FROM line of the Dockerfile for this build.
	From *kapi.ObjectReference `json:"from,omitempty" description:"reference to image stream, image stream tag, or image stream image from which docker image should be pulled, resulting image will be used in the FROM line for the Dockerfile for this build"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty" description:"supported type: dockercfg"`

	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool `json:"noCache,omitempty" description:"if true, indicates that the Docker build must be executed with the --no-cache=true flag"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty" description:"additional environment variables you want to pass into a builder container"`

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool `json:"forcePull,omitempty" description:"forces the source build to pull the image if true"`
}

// SourceBuildStrategy defines input parameters specific to an Source build.
type SourceBuildStrategy struct {
	// From is reference to an DockerImage, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference `json:"from" description:"reference to an image stream, image stream tag, or image stream image from which the Docker image should be pulled"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty" description:"supported type: dockercfg"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty" description:"additional environment variables you want to pass into a builder container"`

	// Scripts is the location of Source scripts
	Scripts string `json:"scripts,omitempty" description:"location of the source scripts"`

	// Incremental flag forces the Source build to do incremental builds if true.
	Incremental bool `json:"incremental,omitempty" description:"forces the source build to do incremental builds if true"`

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool `json:"forcePull,omitempty" description:"forces the source build to pull the image if true"`
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// To defines an optional location to push the output of this build to.
	// Kind must be one of 'ImageStreamTag' or 'DockerImage'.
	// This value will be used to look up a Docker image repository to push to.
	// In the case of an ImageStreamTag, the ImageStreamTag will be looked for in the namespace of
	// the build unless Namespace is specified.
	To *kapi.ObjectReference `json:"to,omitempty" description:"The optional image stream to push the output of this build.  The namespace may be empty, in which case, the image stream will be looked up based on the namespace of the build."`

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
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Spec holds all the input necessary to produce a new build, and the conditions when
	// to trigger them.
	Spec BuildConfigSpec `json:"spec" description:"holds all the input necessary to produce a new build, and the conditions when to trigger them"`
	// Status holds any relevant information about a build config
	Status BuildConfigStatus `json:"status" description:"holds any relevant information about a build config derived by the system"`
}

// BuildConfigSpec describes when and how builds are created
type BuildConfigSpec struct {
	// Triggers determine how new Builds can be launched from a BuildConfig. If no triggers
	// are defined, a new build can only occur as a result of an explicit client build creation.
	Triggers []BuildTriggerPolicy `json:"triggers" description:"determines how new builds can be launched from a build config.  if no triggers are defined, a new build can only occur as a result of an explicit client build creation."`

	// BuildSpec is the desired build specification
	BuildSpec `json:",inline" description:"the desired build specification"`
}

// BuildConfigStatus contains current state of the build config object.
type BuildConfigStatus struct {
	// LastVersion is used to inform about number of last triggered build.
	LastVersion int `json:"lastVersion" description:"used to inform about number of last triggered build"`
}

// WebHookTrigger is a trigger that gets invoked using a webhook type of post
type WebHookTrigger struct {
	// Secret used to validate requests.
	Secret string `json:"secret,omitempty" description:"secret used to validate requests"`
}

// ImageChangeTrigger allows builds to be triggered when an ImageStream changes
type ImageChangeTrigger struct {
	// LastTriggeredImageID is used internally by the ImageChangeController to save last
	// used image ID for build
	LastTriggeredImageID string `json:"lastTriggeredImageID,omitempty" description:"used internally to save last used image ID for build"`

	// From is a reference to an ImageStreamTag that will trigger a build when updated
	// It is optional. If no From is specified, the From image from the build strategy
	// will be used. Only one ImageChangeTrigger with an empty From reference is allowed in
	// a build configuration.
	From *kapi.ObjectReference `json:"from,omitempty" description:"reference to an ImageStreamTag that will trigger the build"`
}

// BuildTriggerPolicy describes a policy for a single trigger that results in a new Build.
type BuildTriggerPolicy struct {
	// Type is the type of build trigger
	Type BuildTriggerType `json:"type" description:"type of build trigger"`

	// GitHubWebHook contains the parameters for a GitHub webhook type of trigger
	GitHubWebHook *WebHookTrigger `json:"github,omitempty" description:"parameters for a GitHub webhook type of trigger"`

	// GenericWebHook contains the parameters for a Generic webhook type of trigger
	GenericWebHook *WebHookTrigger `json:"generic,omitempty" description:"parameters for a Generic webhook type of trigger"`

	// ImageChange contains parameters for an ImageChange type of trigger
	ImageChange *ImageChangeTrigger `json:"imageChange,omitempty" description:"parameters for an ImageChange type of trigger"`
}

// BuildTriggerType refers to a specific BuildTriggerPolicy implementation.
type BuildTriggerType string

const (
	// GitHubWebHookBuildTriggerType represents a trigger that launches builds on
	// GitHub webhook invocations
	GitHubWebHookBuildTriggerType           BuildTriggerType = "GitHub"
	GitHubWebHookBuildTriggerTypeDeprecated BuildTriggerType = "github"

	// GenericWebHookBuildTriggerType represents a trigger that launches builds on
	// generic webhook invocations
	GenericWebHookBuildTriggerType           BuildTriggerType = "Generic"
	GenericWebHookBuildTriggerTypeDeprecated BuildTriggerType = "generic"

	// ImageChangeBuildTriggerType represents a trigger that launches builds on
	// availability of a new version of an image
	ImageChangeBuildTriggerType           BuildTriggerType = "ImageChange"
	ImageChangeBuildTriggerTypeDeprecated BuildTriggerType = "imageChange"

	// ConfigChangeBuildTriggerType will trigger a build on an initial build config creation
	// WARNING: In the future the behavior will change to trigger a build on any config change
	ConfigChangeBuildTriggerType BuildTriggerType = "ConfigChange"
)

// BuildList is a collection of Builds.
type BuildList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of builds
	Items []Build `json:"items" description:"list of builds"`
}

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of build configs
	Items []BuildConfig `json:"items" description:"list of build configs"`
}

// GenericWebHookEvent is the payload expected for a generic webhook post
type GenericWebHookEvent struct {
	// Type is the type of source repository
	Type BuildSourceType `json:"type,omitempty" description:"type of source repository"`

	// Git is the git information if the Type is BuildSourceGit
	Git *GitInfo `json:"git,omitempty" description:"git information if type is git"`
}

// GitInfo is the aggregated git information for a generic webhook post
type GitInfo struct {
	GitBuildSource    `json:",inline"`
	GitSourceRevision `json:",inline"`
}

// BuildLog is the (unused) resource associated with the build log redirector
type BuildLog struct {
	unversioned.TypeMeta `json:",inline"`
}

// BuildRequest is the resource used to pass parameters to build generator
type BuildRequest struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Revision is the information from the source for a specific repo snapshot.
	Revision *SourceRevision `json:"revision,omitempty" description:"information from the source for a specific repo snapshot"`

	// TriggeredByImage is the Image that triggered this build.
	TriggeredByImage *kapi.ObjectReference `json:"triggeredByImage,omitempty" description:"image that triggered this build"`

	// From is the reference to the ImageStreamTag that triggered the build.
	From *kapi.ObjectReference `json:"from,omitempty" description:"ImageStreamTag that triggered this build"`

	// LastVersion (optional) is the LastVersion of the BuildConfig that was used
	// to generate the build. If the BuildConfig in the generator doesn't match, a build will
	// not be generated.
	LastVersion *int `json:"lastVersion,omitempty" description:"LastVersion of the BuildConfig that triggered this build"`
}

// BuildLogOptions is the REST options for a build log
type BuildLogOptions struct {
	unversioned.TypeMeta

	// Follow if true indicates that the build log should be streamed until
	// the build terminates.
	Follow bool `json:"follow,omitempty" description:"if true indicates that the log should be streamed; defaults to false"`

	// NoWait if true causes the call to return immediately even if the build
	// is not available yet. Otherwise the server will wait until the build has started.
	// TODO: Fix the tag to 'noWait' in v2
	NoWait bool `json:"nowait,omitempty" description:"if true indicates that the server should not wait for a log to be available before returning; defaults to false"`
}

// SecretSpec specifies a secret to be included in a build pod and its corresponding mount point
type SecretSpec struct {
	// SecretSource is a reference to the secret
	SecretSource kapi.LocalObjectReference `json:"secretSource" description:"a reference to a secret"`

	// MountPath is the path at which to mount the secret
	MountPath string `json:"mountPath" description:"path within the container at which the secret should be mounted"`
}
