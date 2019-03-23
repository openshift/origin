package v1

const (
	// StatusReasonError is a generic reason for a build error condition.
	StatusReasonError StatusReason = "Error"

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
	StatusReasonCancelBuildFailed StatusReason = "CancelBuildFailed"

	// StatusReasonBuildPodDeleted is an error condition when the build pod is
	// deleted before build completion.
	StatusReasonBuildPodDeleted StatusReason = "BuildPodDeleted"

	// StatusReasonExceededRetryTimeout is an error condition when the build has
	// not completed and retrying the build times out.
	StatusReasonExceededRetryTimeout StatusReason = "ExceededRetryTimeout"

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

	// StatusReasonDockerBuildFailed indicates that the container image build strategy has
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

	// StatusReasonOutOfMemoryKilled indicates that the build pod was killed for its memory consumption
	StatusReasonOutOfMemoryKilled StatusReason = "OutOfMemoryKilled"

	// StatusReasonCannotRetrieveServiceAccount is the reason associated with a failure
	// to look up the service account associated with the BuildConfig.
	StatusReasonCannotRetrieveServiceAccount StatusReason = "CannotRetrieveServiceAccount"

	// StatusReasonBuildPodEvicted is the reason a build fails due to the build pod being evicted
	// from its node
	StatusReasonBuildPodEvicted StatusReason = "BuildPodEvicted"
)
