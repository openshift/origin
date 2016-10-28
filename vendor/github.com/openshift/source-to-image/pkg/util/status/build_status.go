package status

import (
	"github.com/openshift/source-to-image/pkg/api"
)

const (
	// ReasonAssembleFailed is the reason associated with the Assemble script
	// failing.
	ReasonAssembleFailed api.StepFailureReason = "AssembleFailed"
	// ReasonMessageAssembleFailed is the message associated with the Assemble script
	// failing.
	ReasonMessageAssembleFailed api.StepFailureMessage = "Assemble script failed"

	// ReasonPullBuilderImageFailed is the reason associated with failing to pull
	// the builder image.
	ReasonPullBuilderImageFailed api.StepFailureReason = "PullBuilderImageFailed"
	// ReasonMessagePullBuilderImageFailed is the message associated with failing to pull
	// the builder image.
	ReasonMessagePullBuilderImageFailed api.StepFailureMessage = "Failed to pull builder image"

	// ReasonPullRuntimeImageFailed is the reason associated with failing to pull
	// the runtime image.
	ReasonPullRuntimeImageFailed api.StepFailureReason = "PullRuntimeImageFailed"
	// ReasonMessagePullRuntimeImageFailed is the message associated with failing to pull
	// the runtime image.
	ReasonMessagePullRuntimeImageFailed api.StepFailureMessage = "Failed to pull runtime image"

	// ReasonCommitContainerFailed is the reason associated with failing to
	// commit the container to the final image.
	ReasonCommitContainerFailed api.StepFailureReason = "ContainerCommitFailed"
	// ReasonMessageCommitContainerFailed is the message associated with failing to
	// commit the container to the final image.
	ReasonMessageCommitContainerFailed api.StepFailureMessage = "Failed to commit container"

	// ReasonFetchSourceFailed is the reason associated with failing to download
	// the source of the build.
	ReasonFetchSourceFailed api.StepFailureReason = "FetchSourceFailed"
	// ReasonMessageFetchSourceFailed is the message associated with failing to download
	// the source of the build.
	ReasonMessageFetchSourceFailed api.StepFailureMessage = "Failed to fetch source for build"

	// ReasonDockerImageBuildFailed is the reason associated with a failed
	// Docker image build.
	ReasonDockerImageBuildFailed api.StepFailureReason = "DockerImageBuildFailed"
	// ReasonMessageDockerImageBuildFailed is the message associated with a failed
	// Docker image build.
	ReasonMessageDockerImageBuildFailed api.StepFailureMessage = "Docker image build failed"

	// ReasonDockerfileCreateFailed is the reason associated with failing to create a
	// Dockerfile for a build.
	ReasonDockerfileCreateFailed api.StepFailureReason = "DockerFileCreationFailed"
	// ReasonMessageDockerfileCreateFailed is the message associated with failing to create a
	// Dockerfile for a build.
	ReasonMessageDockerfileCreateFailed api.StepFailureMessage = "Failed to create Dockerfile"

	// ReasonInvalidArtifactsMapping is the reason associated with an
	// invalid artifacts mapping of files that need to be copied.
	ReasonInvalidArtifactsMapping api.StepFailureReason = "InvalidArtifactsMapping"
	// ReasonMessageInvalidArtifactsMapping is the message associated with an
	// invalid artifacts mapping of files that need to be copied.
	ReasonMessageInvalidArtifactsMapping api.StepFailureMessage = "Invalid artifacts mapping specified"

	// ReasonArtifactsFetchFailed is the reason associated with a failure to
	// download specified scripts in the application image.
	ReasonArtifactsFetchFailed api.StepFailureReason = "FetchScriptsFailed"
	// ReasonMessageArtifactsFetchFailed is the message associated with a failure to
	// download specified scripts in the application image.
	ReasonMessageArtifactsFetchFailed api.StepFailureMessage = "Failed to fetch scripts specified scripts"

	// ReasonFSOperationFailed is the reason associated with a failed fs
	// operation. Create, remove directory, copy file, etc.
	ReasonFSOperationFailed api.StepFailureReason = "FileSystemOperationFailed"
	// ReasonMessageFSOperationFailed is the message associated with a failed fs
	// operation. Create, remove directory, copy file, etc.
	ReasonMessageFSOperationFailed api.StepFailureMessage = "Failed to perform filesystem operation"

	// ReasonInstallScriptsFailed is the reason associated with a failure to
	// install scripts in the builder image.
	ReasonInstallScriptsFailed api.StepFailureReason = "InstallScriptsFailed"
	// ReasonMessageInstallScriptsFailed is the message associated with a failure to
	// install scripts in the builder image.
	ReasonMessageInstallScriptsFailed api.StepFailureMessage = "Failed to install specified scripts"

	// ReasonGenericS2IBuildFailed is the reason associated with a broad range of
	// failures.
	ReasonGenericS2IBuildFailed api.StepFailureReason = "GenericS2IBuildFailed"
	// ReasonMessageGenericS2iBuildFailed is the message associated with a broad
	// range of failures.
	ReasonMessageGenericS2iBuildFailed api.StepFailureMessage = "Generic S2I Build failure - check S2I logs for details"

	// ReasonTarSourceFailed is the failure reason associated with a failure to
	// tar the current source.
	ReasonTarSourceFailed api.StepFailureReason = "TarSourceFailed"
	// ReasonMessageTarSourceFailed is the message associated with a failure to
	// tar the current source.
	ReasonMessageTarSourceFailed api.StepFailureMessage = "Failed to tar source files"

	// ReasonOnBuildForbidden is the failure reason associated with an image that
	// uses the ONBUILD instruction when it's not allowed.
	ReasonOnBuildForbidden api.StepFailureReason = "OnBuildForbidden"
	// ReasonMessageOnBuildForbidden is the message associated with an image that
	// uses the ONBUILD instruction when it's not allowed.
	ReasonMessageOnBuildForbidden api.StepFailureMessage = "ONBUILD instructions not allowed in this context"
)

// NewFailureReason initializes a new failure reason that contains both the
// reason and a message to be displayed
func NewFailureReason(reason api.StepFailureReason, message api.StepFailureMessage) api.FailureReason {
	return api.FailureReason{
		Reason:  reason,
		Message: message,
	}
}
