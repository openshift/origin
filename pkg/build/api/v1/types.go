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
	// Standard object's metadata.
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
	Resources kapi.ResourceRequirements `json:"resources,omitempty"`

	// PostCommit is a build hook executed after the build output image is
	// committed, before it is pushed to a registry.
	PostCommit BuildPostCommitSpec `json:"postCommit,omitempty"`

	// Optional duration in seconds, counted from the time when a build pod gets
	// scheduled in the system, that the build may be active on a node before the
	// system actively tries to terminate the build; value must be positive integer
	CompletionDeadlineSeconds *int64 `json:"completionDeadlineSeconds,omitempty"`
}

// BuildStatus contains the status of a build
type BuildStatus struct {
	// Phase is the point in the build lifecycle.
	Phase BuildPhase `json:"phase"`

	// Cancelled describes if a cancel event was triggered for the build.
	Cancelled bool `json:"cancelled,omitempty"`

	// Reason is a brief CamelCase string that describes any failure and is meant for machine parsing and tidy display in the CLI.
	Reason StatusReason `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about why the build has this status.
	Message string `json:"message,omitempty"`

	// StartTimestamp is a timestamp representing the server time when this Build started
	// running in a Pod.
	// It is represented in RFC3339 form and is in UTC.
	StartTimestamp *unversioned.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp is a timestamp representing the server time when this Build was
	// finished, whether that build failed or succeeded.  It reflects the time at which
	// the Pod running the Build terminated.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTimestamp *unversioned.Time `json:"completionTimestamp,omitempty"`

	// Duration contains time.Duration object describing build time.
	Duration time.Duration `json:"duration,omitempty"`

	// OutputDockerImageReference contains a reference to the Docker image that
	// will be built by this build. Its value is computed from
	// Build.Spec.Output.To, and should include the registry address, so that
	// it can be used to push and pull the image.
	OutputDockerImageReference string `json:"outputDockerImageReference,omitempty"`

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

// StatusReason is a brief CamelCase string that describes a temporary or
// permanent build error condition, meant for machine parsing and tidy display
// in the CLI.
type StatusReason string

// BuildSourceType is the type of SCM used.
type BuildSourceType string

// Valid values for BuildSourceType.
const (
	//BuildSourceGit instructs a build to use a Git source control repository as the build input.
	BuildSourceGit BuildSourceType = "Git"
	// BuildSourceDockerfile uses a Dockerfile as the start of a build
	BuildSourceDockerfile BuildSourceType = "Dockerfile"
	// BuildSourceBinary indicates the build will accept a Binary file as input.
	BuildSourceBinary BuildSourceType = "Binary"
	// BuildSourceImage indicates the build will accept an image as input
	BuildSourceImage BuildSourceType = "Image"
)

// BuildSource is the SCM used for the build.
type BuildSource struct {
	// Type of build input to accept
	Type BuildSourceType `json:"type"`

	// Binary builds accept a binary as their input. The binary is generally assumed to be a tar,
	// gzipped tar, or zip file depending on the strategy. For Docker builds, this is the build
	// context and an optional Dockerfile may be specified to override any Dockerfile in the
	// build context. For Source builds, this is assumed to be an archive as described above. For
	// Source and Docker builds, if binary.asFile is set the build will receive a directory with
	// a single file. contextDir may be used when an archive is provided. Custom builds will
	// receive this binary as input on STDIN.
	Binary *BinaryBuildSource `json:"binary,omitempty"`

	// Dockerfile is the raw contents of a Dockerfile which should be built. When this option is
	// specified, the FROM may be modified based on your strategy base image and additional ENV
	// stanzas from your strategy environment will be added after the FROM, but before the rest
	// of your Dockerfile stanzas. The Dockerfile source type may be used with other options like
	// git - in those cases the Git repo will have any innate Dockerfile replaced in the context
	// dir.
	Dockerfile *string `json:"dockerfile,omitempty"`

	// Git contains optional information about git build source
	Git *GitBuildSource `json:"git,omitempty"`

	// Images describes a set of images to be used to provide source for the build
	Images []ImageSource `json:"images,omitempty"`

	// ContextDir specifies the sub-directory where the source code for the application exists.
	// This allows to have buildable sources in directory other than root of
	// repository.
	ContextDir string `json:"contextDir,omitempty"`

	// SourceSecret is the name of a Secret that would be used for setting
	// up the authentication for cloning private repository.
	// The secret contains valid credentials for remote repository, where the
	// data's key represent the authentication method to be used and value is
	// the base64 encoded credentials. Supported auth methods are: ssh-privatekey.
	SourceSecret *kapi.LocalObjectReference `json:"sourceSecret,omitempty"`

	// Secrets represents a list of secrets and their destinations that will
	// be used only for the build.
	Secrets []SecretBuildSource `json:"secrets"`
}

// ImageSource describes an image that is used as source for the build
type ImageSource struct {
	// From is a reference to an ImageStreamTag, ImageStreamImage, or DockerImage to
	// copy source from.
	From kapi.ObjectReference `json:"from"`

	// Paths is a list of source and destination paths to copy from the image.
	Paths []ImageSourcePath `json:"paths"`

	// PullSecret is a reference to a secret to be used to pull the image from a registry
	// If the image is pulled from the OpenShift registry, this field does not need to be set.
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty"`
}

// ImageSourcePath describes a path to be copied from a source image and its destination within the build directory.
type ImageSourcePath struct {
	// SourcePath is the absolute path of the file or directory inside the image to
	// copy to the build directory.
	SourcePath string `json:"sourcePath"`

	// DestinationDir is the relative directory within the build directory
	// where files copied from the image are placed.
	DestinationDir string `json:"destinationDir"`
}

// SecretBuildSource describes a secret and its destination directory that will be
// used only at the build time. The content of the secret referenced here will
// be copied into the destination directory instead of mounting.
type SecretBuildSource struct {
	// Secret is a reference to an existing secret that you want to use in your
	// build.
	Secret kapi.LocalObjectReference `json:"secret"`

	// DestinationDir is the directory where the files from the secret should be
	// available for the build time.
	// For the Source build strategy, these will be injected into a container
	// where the assemble script runs. Later, when the script finishes, all files
	// injected will be truncated to zero length.
	// For the Docker build strategy, these will be copied into the build
	// directory, where the Dockerfile is located, so users can ADD or COPY them
	// during docker build.
	DestinationDir string `json:"destinationDir,omitempty"`
}

// BinaryBuildSource describes a binary file to be used for the Docker and Source build strategies,
// where the file will be extracted and used as the build source.
type BinaryBuildSource struct {
	// AsFile indicates that the provided binary input should be considered a single file
	// within the build input. For example, specifying "webapp.war" would place the provided
	// binary as `/webapp.war` for the builder. If left empty, the Docker and Source build
	// strategies assume this file is a zip, tar, or tar.gz file and extract it as the source.
	// The custom strategy receives this binary as standard input. This filename may not
	// contain slashes or be '..' or '.'.
	AsFile string `json:"asFile,omitempty"`
}

// SourceRevision is the revision or commit information from the source for the build
type SourceRevision struct {
	// Type of the build source
	Type BuildSourceType `json:"type"`

	// Git contains information about git-based build source
	Git *GitSourceRevision `json:"git,omitempty"`
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
	URI string `json:"uri"`

	// Ref is the branch/tag/ref to build.
	Ref string `json:"ref,omitempty"`

	// HTTPProxy is a proxy used to reach the git repository over http
	HTTPProxy *string `json:"httpProxy,omitempty"`

	// HTTPSProxy is a proxy used to reach the git repository over https
	HTTPSProxy *string `json:"httpsProxy,omitempty"`
}

// SourceControlUser defines the identity of a user of source control
type SourceControlUser struct {
	// Name of the source control user
	Name string `json:"name,omitempty"`

	// Email of the source control user
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
	// From is reference to an DockerImage, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference `json:"from"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// ExposeDockerSocket will allow running Docker commands (and build Docker images) from
	// inside the Docker container.
	// TODO: Allow admins to enforce 'false' for this option
	ExposeDockerSocket bool `json:"exposeDockerSocket,omitempty"`

	// ForcePull describes if the controller should configure the build pod to always pull the images
	// for the builder or only pull if it is not present locally
	ForcePull bool `json:"forcePull,omitempty"`

	// Secrets is a list of additional secrets that will be included in the build pod
	Secrets []SecretSpec `json:"secrets,omitempty"`

	// BuildAPIVersion is the requested API version for the Build object serialized and passed to the custom builder
	BuildAPIVersion string `json:"buildAPIVersion,omitempty"`
}

// DockerBuildStrategy defines input parameters specific to Docker build.
type DockerBuildStrategy struct {
	// From is reference to an DockerImage, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	// the resulting image will be used in the FROM line of the Dockerfile for this build.
	From *kapi.ObjectReference `json:"from,omitempty"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty"`

	// NoCache if set to true indicates that the docker build must be executed with the
	// --no-cache=true flag
	NoCache bool `json:"noCache,omitempty"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool `json:"forcePull,omitempty"`

	// DockerfilePath is the path of the Dockerfile that will be used to build the Docker image,
	// relative to the root of the context (contextDir).
	DockerfilePath string `json:"dockerfilePath,omitempty"`
}

// SourceBuildStrategy defines input parameters specific to an Source build.
type SourceBuildStrategy struct {
	// From is reference to an DockerImage, ImageStreamTag, or ImageStreamImage from which
	// the docker image should be pulled
	From kapi.ObjectReference `json:"from"`

	// PullSecret is the name of a Secret that would be used for setting up
	// the authentication for pulling the Docker images from the private Docker
	// registries
	PullSecret *kapi.LocalObjectReference `json:"pullSecret,omitempty"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`

	// Scripts is the location of Source scripts
	Scripts string `json:"scripts,omitempty"`

	// Incremental flag forces the Source build to do incremental builds if true.
	Incremental bool `json:"incremental,omitempty"`

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool `json:"forcePull,omitempty"`
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
//        "postCommit": {
//          "script": "rake test --verbose",
//        }
//
//     The above is a convenient form which is equivalent to:
//
//        "postCommit": {
//          "command": ["/bin/sh", "-ic"],
//          "args":    ["rake test --verbose"]
//        }
//
// 2. A command as the image entrypoint:
//
//        "postCommit": {
//          "commit": ["rake", "test", "--verbose"]
//        }
//
//     Command overrides the image entrypoint in the exec form, as documented in
//     Docker: https://docs.docker.com/engine/reference/builder/#entrypoint.
//
// 3. Pass arguments to the default entrypoint:
//
//        "postCommit": {
// 		      "args": ["rake", "test", "--verbose"]
// 	      }
//
//     This form is only useful if the image entrypoint can handle arguments.
//
// 4. Shell script with arguments:
//
//        "postCommit": {
//          "script": "rake test $1",
//          "args":   ["--verbose"]
//        }
//
//     This form is useful if you need to pass arguments that would otherwise be
//     hard to quote properly in the shell script. In the script, $0 will be
//     "/bin/sh" and $1, $2, etc, are the positional arguments from Args.
//
// 5. Command with arguments:
//
//        "postCommit": {
//          "command": ["rake", "test"],
//          "args":    ["--verbose"]
//        }
//
//     This form is equivalent to appending the arguments to the Command slice.
//
// It is invalid to provide both Script and Command simultaneously. If none of
// the fields are specified, the hook is not executed.
type BuildPostCommitSpec struct {
	// Command is the command to run. It may not be specified with Script.
	// This might be needed if the image doesn't have `/bin/sh`, or if you
	// do not want to use a shell. In all other cases, using Script might be
	// more convenient.
	Command []string `json:"command,omitempty"`
	// Args is a list of arguments that are provided to either Command,
	// Script or the Docker image's default entrypoint. The arguments are
	// placed immediately after the command to be run.
	Args []string `json:"args,omitempty"`
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
	Script string `json:"script,omitempty"`
}

// BuildOutput is input to a build strategy and describes the Docker image that the strategy
// should produce.
type BuildOutput struct {
	// To defines an optional location to push the output of this build to.
	// Kind must be one of 'ImageStreamTag' or 'DockerImage'.
	// This value will be used to look up a Docker image repository to push to.
	// In the case of an ImageStreamTag, the ImageStreamTag will be looked for in the namespace of
	// the build unless Namespace is specified.
	To *kapi.ObjectReference `json:"to,omitempty"`

	// PushSecret is the name of a Secret that would be used for setting
	// up the authentication for executing the Docker push to authentication
	// enabled Docker Registry (or Docker Hub).
	PushSecret *kapi.LocalObjectReference `json:"pushSecret,omitempty"`
}

// BuildConfig is a template which can be used to create new builds.
type BuildConfig struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
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

	// BuildSpec is the desired build specification
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

	// From is a reference to an ImageStreamTag that will trigger a build when updated
	// It is optional. If no From is specified, the From image from the build strategy
	// will be used. Only one ImageChangeTrigger with an empty From reference is allowed in
	// a build configuration.
	From *kapi.ObjectReference `json:"from,omitempty"`
}

// BuildTriggerPolicy describes a policy for a single trigger that results in a new Build.
type BuildTriggerPolicy struct {
	// Type is the type of build trigger
	Type BuildTriggerType `json:"type"`

	// GitHubWebHook contains the parameters for a GitHub webhook type of trigger
	GitHubWebHook *WebHookTrigger `json:"github,omitempty"`

	// GenericWebHook contains the parameters for a Generic webhook type of trigger
	GenericWebHook *WebHookTrigger `json:"generic,omitempty"`

	// ImageChange contains parameters for an ImageChange type of trigger
	ImageChange *ImageChangeTrigger `json:"imageChange,omitempty"`
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
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of builds
	Items []Build `json:"items"`
}

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of build configs
	Items []BuildConfig `json:"items"`
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
	unversioned.TypeMeta `json:",inline"`
}

// BuildRequest is the resource used to pass parameters to build generator
type BuildRequest struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Revision is the information from the source for a specific repo snapshot.
	Revision *SourceRevision `json:"revision,omitempty"`

	// TriggeredByImage is the Image that triggered this build.
	TriggeredByImage *kapi.ObjectReference `json:"triggeredByImage,omitempty"`

	// From is the reference to the ImageStreamTag that triggered the build.
	From *kapi.ObjectReference `json:"from,omitempty"`

	// Binary indicates a request to build from a binary provided to the builder
	Binary *BinaryBuildSource `json:"binary,omitempty"`

	// LastVersion (optional) is the LastVersion of the BuildConfig that was used
	// to generate the build. If the BuildConfig in the generator doesn't match, a build will
	// not be generated.
	LastVersion *int `json:"lastVersion,omitempty"`

	// Env contains additional environment variables you want to pass into a builder container
	Env []kapi.EnvVar `json:"env,omitempty"`
}

// BinaryBuildRequestOptions are the options required to fully speficy a binary build request
type BinaryBuildRequestOptions struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// AsFile determines if the binary should be created as a file within the source rather than extracted as an archive
	AsFile string `json:"asFile,omitempty"`

	// TODO: Improve map[string][]string conversion so we can handled nested objects

	// Commit is the value identifying a specific commit
	Commit string `json:"revision.commit,omitempty"`

	// Message is the description of a specific commit
	Message string `json:"revision.message,omitempty"`

	// AuthorName of the source control user
	AuthorName string `json:"revision.authorName,omitempty"`

	// AuthorEmail of the source control user
	AuthorEmail string `json:"revision.authorEmail,omitempty"`

	// CommitterName of the source control user
	CommitterName string `json:"revision.committerName,omitempty"`

	// CommitterEmail of the source control user
	CommitterEmail string `json:"revision.committerEmail,omitempty"`
}

// BuildLogOptions is the REST options for a build log
type BuildLogOptions struct {
	unversioned.TypeMeta `json:",inline"`

	// The container for which to stream logs. Defaults to only container if there is one container in the pod.
	Container string `json:"container,omitempty"`
	// Follow if true indicates that the build log should be streamed until
	// the build terminates.
	Follow bool `json:"follow,omitempty"`
	// Return previous build logs. Defaults to false.
	Previous bool `json:"previous,omitempty"`
	// A relative time in seconds before the current time from which to show logs. If this value
	// precedes the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	SinceSeconds *int64 `json:"sinceSeconds,omitempty"`
	// An RFC3339 timestamp from which to show logs. If this value
	// preceeds the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	SinceTime *unversioned.Time `json:"sinceTime,omitempty"`
	// If true, add an RFC3339 or RFC3339Nano timestamp at the beginning of every line
	// of log output. Defaults to false.
	Timestamps bool `json:"timestamps,omitempty"`
	// If set, the number of lines from the end of the logs to show. If not specified,
	// logs are shown from the creation of the container or sinceSeconds or sinceTime
	TailLines *int64 `json:"tailLines,omitempty"`
	// If set, the number of bytes to read from the server before terminating the
	// log output. This may not display a complete final line of logging, and may return
	// slightly more or slightly less than the specified limit.
	LimitBytes *int64 `json:"limitBytes,omitempty"`

	// NoWait if true causes the call to return immediately even if the build
	// is not available yet. Otherwise the server will wait until the build has started.
	// TODO: Fix the tag to 'noWait' in v2
	NoWait bool `json:"nowait,omitempty"`

	// Version of the build for which to view logs.
	Version *int64 `json:"version,omitempty"`
}

// SecretSpec specifies a secret to be included in a build pod and its corresponding mount point
type SecretSpec struct {
	// SecretSource is a reference to the secret
	SecretSource kapi.LocalObjectReference `json:"secretSource"`

	// MountPath is the path at which to mount the secret
	MountPath string `json:"mountPath"`
}
